package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/core/inference/ollama"
	"github.com/hsme/core/src/core/search"
	"github.com/hsme/core/src/mcp"
	"github.com/hsme/core/src/storage/sqlite"
)

func wrapFuzzySearchResults(results []search.MemorySearchResult) map[string]interface{} {
	if results == nil {
		results = []search.MemorySearchResult{}
	}
	return map[string]interface{}{
		"results": results,
	}
}

func wrapExactSearchResults(results []search.ExactMatchResult) map[string]interface{} {
	if results == nil {
		results = []search.ExactMatchResult{}
	}
	return map[string]interface{}{
		"results": results,
	}
}

type storeContextParams struct {
	Content            string `json:"content"`
	SourceType         string `json:"source_type"`
	SupersedesMemoryID *int64 `json:"supersedes_memory_id"`
	ForceReingest      bool   `json:"force_reingest"`
}

func handleStoreContext(db *sql.DB, params json.RawMessage) (interface{}, error) {
	var p storeContextParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	id, err := indexer.StoreContext(db, p.Content, p.SourceType, p.SupersedesMemoryID, p.ForceReingest)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"memory_id": id, "status": "stored, pending processing"}, nil
}

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		dbPath = "data/engram.db"
	}

	fmt.Fprintf(os.Stderr, "HSME: Usando base de datos: %s\n", dbPath)

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inicializando DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ollamaHost := os.Getenv("OLLAMA_HOST")
	embedModel := os.Getenv("EMBEDDING_MODEL")
	if embedModel == "" {
		embedModel = "nomic-embed-text"
	}

	client := ollama.NewClient(ollamaHost)
	embedder := ollama.NewEmbedder(client, embedModel, 768)

	// Spec §4.2: validar que el embedder activo coincide con la config persistida.
	// Si no hay config (primer arranque) la sembramos; si hay y no coincide,
	// rechazamos el startup con un mensaje claro en vez de dejar que el worker
	// falle chunk por chunk más tarde.
	if err := sqlite.ValidateEmbeddingConfig(db, embedder); err != nil {
		fmt.Fprintf(os.Stderr, "Config de embedding inválida: %v\n", err)
		os.Exit(1)
	}

	srv := mcp.NewServer()

	// Registro de herramienta: search_fuzzy
	srv.RegisterTool("search_fuzzy", "Search memory using hybrid fuzzy matching (Lexical + Semantic)",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
				"limit": map[string]interface{}{"type": "integer", "default": 10},
			},
			"required": []string{"query"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var p struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if p.Limit == 0 {
				p.Limit = 10
			}
			results, err := search.FuzzySearch(context.Background(), db, embedder, p.Query, p.Limit)
			if err != nil {
				return nil, err
			}
			return wrapFuzzySearchResults(results), nil
		},
	)

	// Registro de herramienta: store_context
	srv.RegisterTool("store_context", "Store technical context in memory",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content":              map[string]interface{}{"type": "string"},
				"source_type":          map[string]interface{}{"type": "string"},
				"supersedes_memory_id": map[string]interface{}{"type": []string{"integer", "null"}},
				"force_reingest":       map[string]interface{}{"type": []string{"boolean", "null"}},
			},
			"required": []string{"content", "source_type"},
		},
		func(params json.RawMessage) (interface{}, error) { return handleStoreContext(db, params) },
	)

	// search_exact (spec §14.3): búsqueda léxica exacta sobre FTS5, sin ranking semántico.
	srv.RegisterTool("search_exact", "Exact substring search over memory chunks",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{"type": "string"},
				"limit":   map[string]interface{}{"type": "integer", "default": 10},
			},
			"required": []string{"keyword"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var p struct {
				Keyword string `json:"keyword"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if p.Limit == 0 {
				p.Limit = 10
			}
			results, err := search.ExactSearch(context.Background(), db, p.Keyword, p.Limit)
			if err != nil {
				return nil, err
			}
			return wrapExactSearchResults(results), nil
		},
	)

	// explore_knowledge_graph (spec §14.4): traversal del KG por entidad.
	// entity_name se canonicaliza con la misma pipeline de ingestión (§6.5)
	// antes del lookup, si no el usuario con "Redis" nunca matchearía al
	// nodo guardado como "redis".
	srv.RegisterTool("explore_knowledge_graph", "Trace entity dependencies across the knowledge graph",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_name": map[string]interface{}{"type": "string"},
				"direction":   map[string]interface{}{"type": "string", "enum": []string{"downstream", "upstream", "both"}, "default": "both"},
				"max_depth":   map[string]interface{}{"type": "integer", "default": 5},
				"max_nodes":   map[string]interface{}{"type": "integer", "default": 100},
			},
			"required": []string{"entity_name"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var p struct {
				EntityName string `json:"entity_name"`
				Direction  string `json:"direction"`
				MaxDepth   int    `json:"max_depth"`
				MaxNodes   int    `json:"max_nodes"`
			}
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, err
			}
			if p.Direction == "" {
				p.Direction = "both"
			}
			if p.MaxDepth <= 0 {
				p.MaxDepth = 5
			}
			if p.MaxNodes <= 0 {
				p.MaxNodes = 100
			}
			canonical, _ := indexer.CanonicalizeName(p.EntityName)
			return search.TraceDependencies(context.Background(), db, canonical, p.Direction, p.MaxDepth, p.MaxNodes)
		},
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "Apagando servidor MCP HSME...\n")
		os.Exit(0)
	}()

	fmt.Fprintf(os.Stderr, "HSME: Servidor MCP iniciado con éxito (v1.0.1)\n")
	srv.Serve()
}
