package modules

import (
	"context"
	"os"
	"strings"
	"testing"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/core/search"
	"github.com/hsme/core/src/storage/sqlite"
)

type mockSearchEmbedder struct {
	dim int
}

func (m *mockSearchEmbedder) GenerateVector(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, m.dim)
	t := strings.ToLower(text)
	if t == "semantic" {
		v[0] = 1.0
		return v, nil
	}
	// If text contains "semantic", make it closer to the query vector [1.0, 0, ...]
	if strings.Contains(t, "semantic") {
		v[0] = 0.8
		if strings.Contains(t, "vectors") {
			v[0] = 1.0 // Memory A matches query perfectly
		}
	}
	return v, nil
}

func (m *mockSearchEmbedder) Dimension() int {
	return m.dim
}

func TestFuzzySearchHybrid(t *testing.T) {
	dbPath := "test_search_fuzzy.db"
	defer os.Remove(dbPath)

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	embedder := &mockSearchEmbedder{dim: 768}

	// 1. Ingest memories
	// Memory A: matches lexically and semantically
	idA, _ := indexer.StoreContext(db, "This is a semantic memory document about vectors.", "note", nil, false)
	// Memory B: matches lexically only
	indexer.StoreContext(db, "This is a document about lexical search using FTS5.", "note", nil, false)
	// Memory C: superseded by D
	idC, _ := indexer.StoreContext(db, "Old version of the semantic spec.", "note", nil, false)
	idD, _ := indexer.StoreContext(db, "New version of the semantic spec.", "note", &idC, true)

	// 2. Mock vectorization for Memory A and D (simulate worker completion)
	// We need to manually insert into memory_chunks_vec for testing search
	rows, _ := db.Query("SELECT id, chunk_text FROM memory_chunks WHERE memory_id IN (?, ?)", idA, idD)
	for rows.Next() {
		var chunkID int64
		var text string
		rows.Scan(&chunkID, &text)
		vecData, _ := embedder.GenerateVector(ctx, text)
		blob, _ := vec.SerializeFloat32(vecData)
		// We use a helper or raw SQL since we are testing search, not the worker here
		_, err = db.Exec("INSERT OR REPLACE INTO memory_chunks_vec(rowid, embedding) VALUES(?, ?)", chunkID, blob)
		if err != nil {
			t.Fatalf("Vector insert failed: %v", err)
		}
	}
	rows.Close()

	// 3. Perform Fuzzy Search
	// Querying for "semantic" should rank Memory A high (hybrid match) and Memory D (superseded but match)
	results, err := search.FuzzySearch(ctx, db, embedder, "semantic", 10)
	if err != nil {
		t.Fatalf("FuzzySearch failed: %v", err)
	}

	for i, res := range results {
		t.Logf("Result %d: MemoryID=%d, Score=%f, IsSuperseded=%v", i, res.MemoryID, res.Score, res.IsSuperseded)
	}

	if len(results) == 0 {
		t.Fatal("Expected search results, got none")
	}

	// First result should be Memory A (active and hybrid match)
	if results[0].MemoryID != idA {
		t.Errorf("Expected first result to be memory %d (Memory A), got %d", idA, results[0].MemoryID)
	}

	// Check for superseded flag and penalty
	foundD := false
	for _, res := range results {
		if res.MemoryID == idD {
			foundD = true
		}
		if res.MemoryID == idC && !res.IsSuperseded {
			t.Errorf("Expected memory %d (Memory C) to be marked as superseded", idC)
		}
	}
	if !foundD {
		t.Error("Expected Memory D to be in results")
	}
}
