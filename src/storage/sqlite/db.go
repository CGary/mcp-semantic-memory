package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/mattn/go-sqlite3"
)

func init() {
	// Automatically load sqlite-vec for all new connections
	vec.Auto()

	sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// Enable extension loading
			conn.SetLimit(sqlite3.SQLITE_LIMIT_VARIABLE_NUMBER, 32766)
			return nil
		},
	})
}

const schema = `
-- 1. Global configuration metadata
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 2. Memory document
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    raw_content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'manual',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    superseded_by INTEGER DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    FOREIGN KEY(superseded_by) REFERENCES memories(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_active_hash ON memories(content_hash) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_memories_status        ON memories(status);
CREATE INDEX IF NOT EXISTS idx_memories_superseded_by ON memories(superseded_by);

-- 3. Chunks derived from the document
CREATE TABLE IF NOT EXISTS memory_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id INTEGER NOT NULL,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    token_estimate INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(memory_id, chunk_index),
    FOREIGN KEY(memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_memory_chunks_memory_id ON memory_chunks(memory_id);

-- 4. Lexical index over chunks (FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts USING fts5(
    chunk_text,
    content='memory_chunks',
    content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

-- 4.a Triggers de sincronización memory_chunks <-> memory_chunks_fts.
-- Sin esto, cualquier UPDATE o DELETE sobre memory_chunks deja el índice léxico
-- desincronizado y el FTS devuelve resultados fantasma o pierde filas.
CREATE TRIGGER IF NOT EXISTS memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(rowid, chunk_text) VALUES (new.id, new.chunk_text);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, chunk_text) VALUES ('delete', old.id, old.chunk_text);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, chunk_text) VALUES ('delete', old.id, old.chunk_text);
    INSERT INTO memory_chunks_fts(rowid, chunk_text) VALUES (new.id, new.chunk_text);
END;

-- 5. Vector index over chunks (sqlite-vec)
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_vec USING vec0(
    embedding float[768]
);

-- 6. Asynchronous work queue
CREATE TABLE IF NOT EXISTS async_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id INTEGER NOT NULL,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT DEFAULT NULL,
    leased_until DATETIME DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME DEFAULT NULL,
    UNIQUE(memory_id, task_type),
    FOREIGN KEY(memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_async_tasks_status_lease ON async_tasks(status, leased_until);

-- 7. Graph node catalog
CREATE TABLE IF NOT EXISTS kg_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    canonical_name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, canonical_name)
);

CREATE INDEX IF NOT EXISTS idx_kg_nodes_canonical ON kg_nodes(canonical_name);

-- 8. Edge evidence
CREATE TABLE IF NOT EXISTS kg_edge_evidence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_node_id INTEGER NOT NULL,
    target_node_id INTEGER NOT NULL,
    relation_type TEXT NOT NULL,
    memory_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_node_id, target_node_id, relation_type, memory_id),
    FOREIGN KEY(source_node_id) REFERENCES kg_nodes(id),
    FOREIGN KEY(target_node_id) REFERENCES kg_nodes(id),
    FOREIGN KEY(memory_id) REFERENCES memories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_edge_source    ON kg_edge_evidence(source_node_id);
CREATE INDEX IF NOT EXISTS idx_edge_target    ON kg_edge_evidence(target_node_id);
CREATE INDEX IF NOT EXISTS idx_edge_memory    ON kg_edge_evidence(memory_id);
`

func InitDB(path string) (*sql.DB, error) {
	// _txlock=immediate hace que BeginTx emita BEGIN IMMEDIATE en vez de BEGIN DEFERRED.
	// Con eso el write-lock se toma upfront y las escrituras concurrentes (ej. dos
	// store_context con el mismo contenido) serializan limpias en vez de colisionar
	// en el unique index al commit.
	dsn := fmt.Sprintf("file:%s?_txlock=immediate", path)
	db, err := sql.Open("sqlite3_custom", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Acotar el pool de conexiones. Con MaxOpenConns ilimitado y goroutines
	// concurrentes (ver mcp/handler.go: `go s.handleRequest`), SQLite bajo WAL
	// puede devolver `database is locked` cuando muchas conexiones compiten por
	// el writer. 4 permite lectores concurrentes sin disparar contention.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)

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
