package search

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// embedTimeout acota el tiempo que una búsqueda MCP puede esperar a Ollama.
// El HTTPClient del cliente Ollama tiene timeout de 2 minutos (pensado para
// ingestión batch), pero en un search síncrono el cliente MCP corta mucho antes
// y pierde la sesión. Si se excede, caemos a búsqueda léxica pura.
const embedTimeout = 10 * time.Second

// sanitizeFTS convierte una consulta en lenguaje natural a una expresión FTS5 segura.
// Tokeniza por espacios y envuelve cada término entre comillas dobles (escapando comillas internas),
// neutralizando operadores FTS5 como `:`, `-`, `*`, paréntesis y comillas sueltas.
func sanitizeFTS(q string) string {
	fields := strings.Fields(q)
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " ")
}

type Embedder interface {
	GenerateVector(ctx context.Context, text string) ([]float32, error)
	Dimension() int
	ModelID() string
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

	// 2. Vector Search (sqlite-vec) — embedding con timeout acotado.
	var vectorResults []SearchResult
	if embedder != nil {
		embedCtx, cancel := context.WithTimeout(ctx, embedTimeout)
		vector, err := embedder.GenerateVector(embedCtx, query)
		cancel()
		if err == nil {
			vectorResults, err = VectorSearch(ctx, db, vector, limit*2)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error en búsqueda vectorial: %v\n", err)
				vectorResults = nil
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error generando vector para búsqueda (timeout=%s, degradando a léxica): %v\n", embedTimeout, err)
		}
	}

	// 3. Fusion using RRF at chunk level
	fusedChunks := RRF(limit*2, lexicalResults, vectorResults)
	if len(fusedChunks) == 0 {
		return nil, nil
	}

	// 4. Batch-fetch todos los chunks fusionados de una (JOIN contra memories
	// para traer el status en el mismo roundtrip). Antes esto era un SELECT por
	// chunk → ~20 queries para limit=10. Ahora es UNA.
	chunkIDs := make([]any, len(fusedChunks))
	for i, c := range fusedChunks {
		chunkIDs[i] = c.ID
	}
	placeholders := strings.Repeat(",?", len(fusedChunks))[1:]
	chunkRows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.id, c.memory_id, c.chunk_index, c.chunk_text, m.status
		  FROM memory_chunks c
		  JOIN memories m ON m.id = c.memory_id
		 WHERE c.id IN (%s)`, placeholders), chunkIDs...)
	if err != nil {
		return nil, fmt.Errorf("batch chunk lookup: %w", err)
	}
	type chunkMeta struct {
		memoryID     int64
		chunkIndex   int
		chunkText    string
		memoryStatus string
	}
	chunkByID := make(map[int64]chunkMeta, len(fusedChunks))
	for chunkRows.Next() {
		var id int64
		var cm chunkMeta
		if err := chunkRows.Scan(&id, &cm.memoryID, &cm.chunkIndex, &cm.chunkText, &cm.memoryStatus); err != nil {
			chunkRows.Close()
			return nil, err
		}
		chunkByID[id] = cm
	}
	chunkRows.Close()

	// Iteramos fusedChunks en orden RRF para que los highlights salgan en orden
	// de relevancia (el map del batch query no tiene orden garantizado).
	memoryScores := make(map[int64]float64)
	memoryHighlights := make(map[int64][]ChunkHighlight)
	memoryStatus := make(map[int64]string)
	for _, chunk := range fusedChunks {
		cm, ok := chunkByID[chunk.ID]
		if !ok {
			continue
		}
		memoryStatus[cm.memoryID] = cm.memoryStatus
		if chunk.Score > memoryScores[cm.memoryID] {
			memoryScores[cm.memoryID] = chunk.Score
		}
		if len(memoryHighlights[cm.memoryID]) < 3 {
			memoryHighlights[cm.memoryID] = append(memoryHighlights[cm.memoryID], ChunkHighlight{
				ChunkID:    chunk.ID,
				ChunkIndex: cm.chunkIndex,
				Text:       cm.chunkText,
			})
		}
	}

	// 5. Coverage vectorial — UNA query GROUP BY en vez de 2 queries por memoria.
	coverageByMemory := make(map[int64]string, len(memoryScores))
	if len(memoryScores) > 0 {
		memIDs := make([]any, 0, len(memoryScores))
		for id := range memoryScores {
			memIDs = append(memIDs, id)
		}
		memPlaceholders := strings.Repeat(",?", len(memIDs))[1:]
		covRows, err := db.QueryContext(ctx, fmt.Sprintf(`
			SELECT c.memory_id, COUNT(c.id) AS total,
			       SUM(CASE WHEN v.rowid IS NOT NULL THEN 1 ELSE 0 END) AS with_vec
			  FROM memory_chunks c
			  LEFT JOIN memory_chunks_vec v ON v.rowid = c.id
			 WHERE c.memory_id IN (%s)
			 GROUP BY c.memory_id`, memPlaceholders), memIDs...)
		if err != nil {
			return nil, fmt.Errorf("batch coverage lookup: %w", err)
		}
		for covRows.Next() {
			var mid int64
			var total, withVec int
			if err := covRows.Scan(&mid, &total, &withVec); err != nil {
				covRows.Close()
				return nil, err
			}
			switch {
			case total > 0 && withVec == total:
				coverageByMemory[mid] = "complete"
			case withVec > 0:
				coverageByMemory[mid] = "partial"
			default:
				coverageByMemory[mid] = "none"
			}
		}
		covRows.Close()
	}

	// 6. Build final results
	var results []MemorySearchResult
	for memoryID, score := range memoryScores {
		isSuperseded := memoryStatus[memoryID] == "superseded"
		finalScore := score
		if isSuperseded {
			finalScore *= 0.5 // Default penalty §12.4
		}
		coverage, ok := coverageByMemory[memoryID]
		if !ok {
			coverage = "none"
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
	safe := sanitizeFTS(query)
	if safe == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, "SELECT rowid, rank FROM memory_chunks_fts WHERE chunk_text MATCH ? ORDER BY rank LIMIT ?", safe, limit)
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

	rows, err := db.QueryContext(ctx, "SELECT rowid FROM memory_chunks_vec WHERE embedding MATCH ? LIMIT ?", blob, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.ID); err != nil {
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
