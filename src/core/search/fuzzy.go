package search

import (
	"context"
	"database/sql"
	"sort"
)

type SearchResult struct {
	ID    int64
	Score float64
}

func FuzzySearch(ctx context.Context, db *sql.DB, query string, limit int) ([]SearchResult, error) {
	// memory_chunks_fts uses rowid that matches memory_chunks.id
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
		results = append(results, res)
	}
	return results, nil
}

func ExactSearch(ctx context.Context, db *sql.DB, keyword string, limit int) ([]SearchResult, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, 1.0 FROM memory_chunks WHERE chunk_text LIKE ? LIMIT ?", "%"+keyword+"%", limit)
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

// RRF (Reciprocal Rank Fusion) implements the fusion of multiple search result sets.
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
