# HSME: Hybrid Semantic Memory Engine

HSME is a local-first, high-performance memory engine designed to provide long-term context and semantic reasoning to AI agents. It operates as a Model Context Protocol (MCP) server, integrating advanced vector search with traditional lexical indexing.

## 🚀 Key Features

*   **Hybrid Retrieval**: Combines SQLite FTS5 (Lexical) and `sqlite-vec` (Semantic) using Reciprocal Rank Fusion (RRF).
*   **Background Enrichment**: Asynchronous worker that processes embeddings and extracts knowledge graphs using local LLMs (Ollama).
*   **Causal Traceability**: A Knowledge Graph layer to track dependencies and relations between technical entities.
*   **Privacy-First**: 100% local execution. No data leaves the host environment.

---

## 🏗️ Architecture

HSME is built with **Go** and leverages **SQLite** as its primary storage engine. 

1.  **Ingestion**: Documents are hashed, chunked, and stored synchronously.
2.  **Inference**: An internal worker polls pending tasks to generate vectors via `nomic-embed-text` and extract entities via `phi3.5`.
3.  **Retrieval**: Exposes tools via the MCP protocol for agents to store and query context.

---

## 💾 Data Persistence & Integrity

The state of HSME resides entirely in a SQLite database. For the engine to function and maintain consistency, it is critical to understand its physical structure.

### Anatomy of the Database
By default, the data directory contains three files:
1.  **`engram.db`**: The main database file containing all structured data, FTS5 indexes, and vectors.
2.  **`engram.db-wal`**: The Write-Ahead Log. This file contains committed transactions not yet merged into the main `.db`. **Never delete or ignore this file.**
3.  **`engram.db-shm`**: Shared memory file used for concurrent access.

### 🛡️ Backup Strategy
Because HSME operates in WAL mode, a simple file copy while the server is running might result in a corrupted backup.

#### 1. Atomic Online Backup (Recommended)
Use the SQLite backup API to create a consistent snapshot without stopping the server:
```bash
sqlite3 /path/to/data/engram.db ".backup /path/to/backups/hsme_backup_$(date +%Y%m%d).db"
```

#### 2. Offline Backup
If the MCP server is disconnected, copy all three files (`.db`, `.db-wal`, `.db-shm`) to your backup destination.

### 🔄 Disaster Recovery
To restore HSME on a new system:
1.  **Source Code**: Clone the repository and build the binary (`go build -tags="..."`).
2.  **Dependencies**: Install Ollama and pull the required models (`nomic-embed-text`, `phi3.5`).
3.  **Data**: Place your `engram.db` backup into the data directory defined by `SQLITE_DB_PATH`.
4.  **Launch**: Restart the Gemini CLI or your MCP host.

---

## 🛠️ Build & Configuration

### Build Tags
HSME requires specific CGO build tags to enable SQLite extensions:
```bash
go build -tags="sqlite_load_extension sqlite_fts5" -o hsme ./cmd/server
```

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `SQLITE_DB_PATH` | Absolute path to the .db file | `./engram.db` |
| `OLLAMA_HOST` | URL of the Ollama service | `http://localhost:11434` |
| `EMBEDDING_MODEL` | Model for vector generation | `nomic-embed-text` |
| `EXTRACTION_MODEL` | Model for graph extraction | `phi3.5` |

---

## 🔧 MCP Tools Exposes

*   **`store_context`**: Ingests new text into the memory engine.
*   **`search_fuzzy`**: Performs hybrid search (Semantic + Lexical).
*   **`search_exact`**: Performs keyword-based lookups.
*   **`trace_dependencies`**: Navigates the knowledge graph for entity relations.

---

## ⚠️ Critical Warning
**Vector Dimension Stability**: The database is initialized for **768-dimensional** vectors. Changing the `EMBEDDING_MODEL` to a model with different dimensions will render the existing `memory_chunks_vec` table invalid and require a full re-index or a fresh database.
