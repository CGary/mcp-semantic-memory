package search

import (
	"context"
	"database/sql"
	"sort"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type Embedder interface {
	GenerateVector(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type ChunkHighlight struct {
	ChunkID    int64  `json:"chunk_id"`
	ChunkIndex int    `json:"chunk_index"`
	Text       string `json:"text"`
}

type MemorySearchResult struct {
	MemoryID       int64            `json:"memory_id"`
	Score          float64          `json:"score"`
	IsSuperseded   bool             `json:"is_superseded"`
	VectorCoverage string           `json:"vector_coverage"` // "complete|partial|none"
	Highlights     []ChunkHighlight `json:"highlights"`
}

type SearchResult struct {
	ID    int64
	Score float64
}

func FuzzySearch(ctx context.Context, db *sql.DB, embedder Embedder, query string, limit int) ([]MemorySearchResult, error) {
	// 1. Lexical Search (FTS5)
	lexicalResults, err := LexicalSearch(ctx, db, query, limit*2)
	if err != nil {
		return nil, err
	}

	// 2. Vector Search (sqlite-vec)
	var vectorResults []SearchResult
	if embedder != nil {
		vector, err := embedder.GenerateVector(ctx, query)
		if err == nil {
			vectorResults, err = VectorSearch(ctx, db, vector, limit*2)
			if err != nil {
				// Log error but continue with lexical results
				vectorResults = nil
			}
		}
	}

	// 3. Fusion using RRF at chunk level
	fusedChunks := RRF(limit*2, lexicalResults, vectorResults)

	// 4. Aggregate by memory_id
	memoryScores := make(map[int64]float64)
	memoryHighlights := make(map[int64][]ChunkHighlight)

	for _, chunk := range fusedChunks {
		var memoryID int64
		var chunkIndex int
		var chunkText string
		err := db.QueryRowContext(ctx, "SELECT memory_id, chunk_index, chunk_text FROM memory_chunks WHERE id = ?", chunk.ID).Scan(&memoryID, &chunkIndex, &chunkText)
		if err != nil {
			continue
		}

		// Document score is the maximum RRF score among its chunks
		if chunk.Score > memoryScores[memoryID] {
			memoryScores[memoryID] = chunk.Score
		}

		// Collect highlights (limit to 3 per memory for now)
		if len(memoryHighlights[memoryID]) < 3 {
			memoryHighlights[memoryID] = append(memoryHighlights[memoryID], ChunkHighlight{
				ChunkID:    chunk.ID,
				ChunkIndex: chunkIndex,
				Text:       chunkText,
			})
		}
	}

	// 5. Convert to final results and apply penalties
	var results []MemorySearchResult
	for memoryID, score := range memoryScores {
		var status string
		err := db.QueryRowContext(ctx, "SELECT status FROM memories WHERE id = ?", memoryID).Scan(&status)
		if err != nil {
			continue
		}

		isSuperseded := status == "superseded"
		finalScore := score
		if isSuperseded {
			finalScore *= 0.5 // Default penalty §12.4
		}

		// Check vector coverage
		var totalChunks, chunksWithVectors int
		_ = db.QueryRowContext(ctx, "SELECT count(*) FROM memory_chunks WHERE memory_id = ?", memoryID).Scan(&totalChunks)
		_ = db.QueryRowContext(ctx, "SELECT count(*) FROM memory_chunks_vec v JOIN memory_chunks c ON v.rowid = c.id WHERE c.memory_id = ?", memoryID).Scan(&chunksWithVectors)

		coverage := "none"
		if chunksWithVectors == totalChunks && totalChunks > 0 {
			coverage = "complete"
		} else if chunksWithVectors > 0 {
			coverage = "partial"
		}

		results = append(results, MemorySearchResult{
			MemoryID:       memoryID,
			Score:          finalScore,
			IsSuperseded:   isSuperseded,
			VectorCoverage: coverage,
			Highlights:     memoryHighlights[memoryID],
		})
	}

	// 6. Sort by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func LexicalSearch(ctx context.Context, db *sql.DB, query string, limit int) ([]SearchResult, error) {
	rows, err := db.QueryContext(ctx, "SELECT rowid, rank FROM memory_chunks_fts WHERE chunk_text MATCH ? ORDER BY rank LIMIT ?", query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.ID, &res.Score); err != nil {
			return nil, err
		}
		// FTS5 rank: lower is better. RRF expects position-based ranking.
		// We just return them sorted, RRF uses the index.
		results = append(results, res)
	}
	return results, nil
}

func VectorSearch(ctx context.Context, db *sql.DB, vector []float32, limit int) ([]SearchResult, error) {
	blob, err := vec.SerializeFloat32(vector)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "SELECT rowid, distance FROM memory_chunks_vec WHERE embedding MATCH ? LIMIT ?", blob, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.ID, &res.Score); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func ExactSearch(ctx context.Context, db *sql.DB, keyword string, limit int) ([]MemorySearchResult, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT m.id, m.status
		FROM memories m
		JOIN memory_chunks c ON m.id = c.memory_id
		WHERE c.chunk_text LIKE ?
		LIMIT ?
	`, "%"+keyword+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MemorySearchResult
	for rows.Next() {
		var res MemorySearchResult
		var status string
		if err := rows.Scan(&res.MemoryID, &status); err != nil {
			return nil, err
		}
		res.IsSuperseded = status == "superseded"
		res.Score = 1.0
		results = append(results, res)
	}
	return results, nil
}

func RRF(limit int, resultSets ...[]SearchResult) []SearchResult {
	scores := make(map[int64]float64)
	k := 60.0
	for _, resultSet := range resultSets {
		for i, res := range resultSet {
			scores[res.ID] += 1.0 / (k + float64(i+1))
		}
	}

	var fused []SearchResult
	for id, score := range scores {
		fused = append(fused, SearchResult{ID: id, Score: score})
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	if len(fused) > limit {
		return fused[:limit]
	}
	return fused
}
