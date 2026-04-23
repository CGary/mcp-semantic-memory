package indexer

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/hsme/core/src/core/models"
)

// StoreContext ingests a new memory document.
func StoreContext(db *sql.DB, content string, sourceType string, forceReingest bool) (int64, error) {
	// 1. Compute hash for deduplication
	hash := ComputeHash(content)

	// 2. Check for deduplication
	if !forceReingest {
		var existingID int64
		err := db.QueryRow("SELECT id FROM memories WHERE content_hash = ?", hash).Scan(&existingID)
		if err == nil {
			return existingID, nil
		} else if err != sql.ErrNoRows {
			return 0, fmt.Errorf("failed to check for existing content: %w", err)
		}
	}

	// 3. Start transaction
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Insert into memories
	res, err := tx.Exec(`
		INSERT INTO memories (raw_content, content_hash, source_type, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, content, hash, sourceType, "pending", time.Now(), time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to insert memory: %w", err)
	}

	memoryID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	// 5. Split into chunks and insert
	chunks := Split(content, sourceType)
	for i, chunkText := range chunks {
		chunkRes, err := tx.Exec(`
			INSERT INTO memory_chunks (memory_id, chunk_index, chunk_text, token_estimate)
			VALUES (?, ?, ?, ?)
		`, memoryID, i, chunkText, estimateTokens(chunkText))
		if err != nil {
			return 0, fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}

		chunkID, err := chunkRes.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get chunk last insert id: %w", err)
		}

		// 6. Explicitly sync memory_chunks_fts
		// Note: db.go has triggers, so this might be redundant or fail if not handled.
		// We use INSERT OR IGNORE or just allow it to fail silently if it's already there.
		_, _ = tx.Exec(`
			INSERT OR IGNORE INTO memory_chunks_fts(rowid, chunk_text) VALUES (?, ?)
		`, chunkID, chunkText)
	}

	// 7. Enqueue async tasks (T007)
	_, err = tx.Exec(`
		INSERT INTO async_tasks (memory_id, task_type, status)
		VALUES (?, ?, ?)
	`, memoryID, "embed", "pending")
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue embed task: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO async_tasks (memory_id, task_type, status)
		VALUES (?, ?, ?)
	`, memoryID, "graph_extract", "pending")
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue graph_extract task: %w", err)
	}

	// 8. Commit
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return memoryID, nil
}
