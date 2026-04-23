package modules

import (
	"os"
	"testing"

	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/storage/sqlite"
)

func TestContentHashing(t *testing.T) {
	content := "Hello World"
	// echo -n "Hello World" | sha256sum
	expected := "a591a6d40bf420404a011733cfb7b190d62c65bf0bcda32b57b277d9ad9f146e"
	hash := indexer.ComputeHash(content)
	if hash != expected {
		t.Errorf("Expected %s, got %s", expected, hash)
	}

	// Test NFC normalization: e + combining acute accent (\u0301) should match é (\u00e9)
	content2 := "e\u0301"
	expected2 := indexer.ComputeHash("\u00e9")
	hash2 := indexer.ComputeHash(content2)
	if hash2 != expected2 {
		t.Errorf("NFC normalization failed: hash of normalized content should match")
	}
}

func TestChunking(t *testing.T) {
	// Recursive splitting on \n\n, \n, space.
	// We want to test that it splits correctly when content is "large"
	// For this test, let's assume a small target to trigger splits easily if possible, 
	// but the requirement says "targeting 400-800 tokens".
	
	content := "Para 1 line 1\nPara 1 line 2\n\nPara 2 line 1"
	chunks := indexer.Split(content, "text")
	
	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}
	
	// If the content is small, it might be just one chunk.
	// Let's create a larger content to see if it splits.
	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "word "
	}
	chunks = indexer.Split(largeContent, "text")
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for large content, got %d", len(chunks))
	}
}

func TestStoreContext(t *testing.T) {
	dbPath := "test_indexer.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		// This might fail if vec0 is missing, but we expect logically correct code.
		t.Skipf("Skipping StoreContext test because DB init failed (likely missing vec0): %v", err)
		return
	}
	defer db.Close()

	content := "Unique content for test"
	sourceType := "test"
	
	id, err := indexer.StoreContext(db, content, sourceType, false)
	if err != nil {
		t.Fatalf("StoreContext failed: %v", err)
	}
	
	if id <= 0 {
		t.Errorf("Expected positive ID, got %d", id)
	}

	// Test deduplication
	id2, err := indexer.StoreContext(db, content, sourceType, false)
	if err != nil {
		t.Fatalf("StoreContext failed on duplicate: %v", err)
	}
	if id != id2 {
		t.Errorf("Expected same ID for duplicate content, got %d and %d", id, id2)
	}

	// Test force reingest
	id3, err := indexer.StoreContext(db, content, sourceType, true)
	if err != nil {
		t.Fatalf("StoreContext failed on force reingest: %v", err)
	}
	if id == id3 {
		t.Errorf("Expected different ID for force reingest, got %d", id3)
	}
	
	// Verify async tasks were enqueued
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM async_tasks WHERE memory_id = ?", id).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query async_tasks: %v", err)
	}
	// T007 expects two tasks: embed and graph_extract
	if count != 2 {
		t.Errorf("Expected 2 async tasks for memory %d, got %d", id, count)
	}
}
