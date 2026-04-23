package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/core/search"
	"github.com/hsme/core/src/core/worker"
	"github.com/hsme/core/src/mcp"
	"github.com/hsme/core/src/storage/sqlite"
)

// Mock implementations for worker interfaces
type mockEmbedder struct{}

func (m *mockEmbedder) GenerateVector(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 768), nil
}
func (m *mockEmbedder) Dimension() int { return 768 }

type mockExtractor struct{}

func (m *mockExtractor) ExtractEntities(ctx context.Context, text string) (worker.KnowledgeGraph, error) {
	return worker.KnowledgeGraph{}, nil
}

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

	// Initialize worker
	w := worker.NewWorker(db, &mockEmbedder{}, &mockExtractor{})
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
	srv.RegisterTool("store_context", func(params json.RawMessage) (interface{}, error) {
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
	})

	srv.RegisterTool("search_fuzzy", func(params json.RawMessage) (interface{}, error) {
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
		return search.FuzzySearch(ctx, db, &mockEmbedder{}, args.Query, args.Limit)
	})

	srv.RegisterTool("search_exact", func(params json.RawMessage) (interface{}, error) {
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
	})

	srv.RegisterTool("trace_dependencies", func(params json.RawMessage) (interface{}, error) {
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
	})

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		os.Exit(0)
	}()

	// Start MCP server loop
	fmt.Fprintf(os.Stderr, "MCP server starting...\n")
	srv.Serve()
}
