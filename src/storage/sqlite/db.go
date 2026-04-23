package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// Enable extension loading
			conn.SetLimit(sqlite3.SQLITE_LIMIT_VARIABLE_NUMBER, 32766)
			
			// Load vec0 extension
			// We try common names for the extension
			err := conn.LoadExtension("vec0", "sqlite3_vec_init")
			if err != nil {
				// Fallback to just "vec0"
				_ = conn.LoadExtension("vec0", "")
			}
			return nil
		},
	})
}

const schema = `
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    raw_content TEXT,
    content_hash TEXT UNIQUE,
    source_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    superseded_by INTEGER REFERENCES memories(id),
    status TEXT
);

CREATE TABLE IF NOT EXISTS memory_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id INTEGER REFERENCES memories(id),
    chunk_index INTEGER,
    chunk_text TEXT,
    token_estimate INTEGER
);

-- FTS5 table for memory chunks
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts USING fts5(
    chunk_text,
    content='memory_chunks',
    content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

-- Trigger to keep FTS index updated
CREATE TRIGGER IF NOT EXISTS memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
  INSERT INTO memory_chunks_fts(rowid, chunk_text) VALUES (new.id, new.chunk_text);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
  INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, chunk_text) VALUES('delete', old.id, old.chunk_text);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
  INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, chunk_text) VALUES('delete', old.id, old.chunk_text);
  INSERT INTO memory_chunks_fts(rowid, chunk_text) VALUES (new.id, new.chunk_text);
END;

-- Vector table for memory chunks
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_vec USING vec0(
    embedding float[768]
);

CREATE TABLE IF NOT EXISTS async_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id INTEGER REFERENCES memories(id),
    task_type TEXT,
    status TEXT,
    attempt_count INTEGER DEFAULT 0,
    last_error TEXT,
    leased_until DATETIME
);

CREATE TABLE IF NOT EXISTS kg_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT,
    canonical_name TEXT,
    display_name TEXT
);

CREATE TABLE IF NOT EXISTS kg_edge_evidence (
    source_node_id INTEGER REFERENCES kg_nodes(id),
    target_node_id INTEGER REFERENCES kg_nodes(id),
    relation_type TEXT,
    memory_id INTEGER REFERENCES memories(id),
    PRIMARY KEY (source_node_id, target_node_id, memory_id)
);
`

func InitDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3_custom", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Apply schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return db, nil
}
