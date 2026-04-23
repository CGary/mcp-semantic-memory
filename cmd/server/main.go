package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/core/inference/ollama"
	"github.com/hsme/core/src/core/search"
	"github.com/hsme/core/src/core/worker"
	"github.com/hsme/core/src/mcp"
	"github.com/hsme/core/src/storage/sqlite"
)

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		dbPath = "engram.db"
	}

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	ollamaHost := os.Getenv("OLLAMA_HOST")
	embedModel := os.Getenv("EMBEDDING_MODEL")
	extractModel := os.Getenv("EXTRACTION_MODEL")

	client := ollama.NewClient(ollamaHost)
	embedder := ollama.NewEmbedder(client, embedModel, 768)
	extractor := ollama.NewExtractor(client, extractModel)

	// Initialize worker
	w := worker.NewWorker(db, embedder, extractor)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				task, err := w.LeaseNextTask(ctx)
				if err != nil {
					continue
				}
				if task != nil {
					w.ExecuteTask(ctx, task)
				}
			}
		}
	}()

	// Initialize MCP server
	srv := mcp.NewServer()

	// Register tools
	srv.RegisterTool(
		"store_context",
		"Store technical context in memory",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content":              map[string]interface{}{"type": "string"},
				"source_type":          map[string]interface{}{"type": "string"},
				"supersedes_memory_id": map[string]interface{}{"type": "integer"},
				"force_reingest":       map[string]interface{}{"type": "boolean"},
			},
			"required": []string{"content", "source_type"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var args struct {
				Content            string `json:"content"`
				SourceType         string `json:"source_type"`
				SupersedesMemoryID *int64 `json:"supersedes_memory_id"`
				ForceReingest      bool   `json:"force_reingest"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, err
			}
			id, err := indexer.StoreContext(db, args.Content, args.SourceType, args.SupersedesMemoryID, args.ForceReingest)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"memory_id": id,
				"status":    "stored",
			}, nil
		},
	)

	srv.RegisterTool(
		"search_fuzzy",
		"Search memory using hybrid fuzzy matching (Lexical + Semantic)",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
				"limit": map[string]interface{}{"type": "integer"},
			},
			"required": []string{"query"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, err
			}
			if args.Limit == 0 {
				args.Limit = 10
			}
			return search.FuzzySearch(ctx, db, embedder, args.Query, args.Limit)
		},
	)

	srv.RegisterTool(
		"search_exact",
		"Search memory using exact keyword matching",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{"type": "string"},
				"limit":   map[string]interface{}{"type": "integer"},
			},
			"required": []string{"keyword"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var args struct {
				Keyword string `json:"keyword"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, err
			}
			if args.Limit == 0 {
				args.Limit = 10
			}
			return search.ExactSearch(ctx, db, args.Keyword, args.Limit)
		},
	)

	srv.RegisterTool(
		"trace_dependencies",
		"Trace entity dependencies in the knowledge graph",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entity_name": map[string]interface{}{"type": "string"},
				"direction":   map[string]interface{}{"type": "string", "enum": []string{"downstream", "upstream", "both"}},
				"max_depth":   map[string]interface{}{"type": "integer"},
			},
			"required": []string{"entity_name"},
		},
		func(params json.RawMessage) (interface{}, error) {
			var args struct {
				EntityName string `json:"entity_name"`
				Direction  string `json:"direction"`
				MaxDepth   int    `json:"max_depth"`
			}
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, err
			}
			if args.MaxDepth == 0 {
				args.MaxDepth = 3
			}
			if args.Direction == "" {
				args.Direction = "both"
			}
			return search.TraceDependencies(ctx, db, args.EntityName, args.Direction, args.MaxDepth)
		},
	)

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		os.Exit(0)
	}()

	// Start MCP server loop
	srv.Serve()
}
