package modules

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hsme/core/src/storage/sqlite"
	"github.com/hsme/core/src/core/worker"
	_ "github.com/mattn/go-sqlite3"
)

type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) GenerateVector(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, m.dim), nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

type mockGraphExtractor struct{}

func (m *mockGraphExtractor) ExtractEntities(ctx context.Context, text string) (worker.KnowledgeGraph, error) {
	return worker.KnowledgeGraph{
		Nodes: []worker.Node{
			{Type: "ENTITY", Name: "Entity A"},
			{Type: "ENTITY", Name: "Entity B"},
		},
		Edges: []worker.Edge{
			{Source: "Entity A", Target: "Entity B", Relation: "DEPENDS_ON"},
		},
	}, nil
}

func TestLeasingLogic(t *testing.T) {
	dbPath := "test_worker_leasing.db"
	defer os.Remove(dbPath)

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert memories for foreign key constraints
	_, err = db.Exec("INSERT INTO memories (id, raw_content, content_hash, status) VALUES (1, 'content', 'hash1', 'active')")
	if err != nil {
		t.Fatalf("Failed to insert memory: %v", err)
	}

	w := worker.NewWorker(db, &mockEmbedder{dim: 768}, &mockGraphExtractor{})

	// 1. Test leasing a pending task
	_, err = db.Exec("INSERT INTO async_tasks (memory_id, task_type, status) VALUES (1, 'embed', 'pending')")
	if err != nil {
		t.Fatalf("Failed to insert task: %v", err)
	}

	task, err := w.LeaseNextTask(context.Background())
	if err != nil {
		t.Fatalf("Failed to lease task: %v", err)
	}
	if task == nil {
		t.Fatal("Expected task to be leased, got nil")
	}
	if task.Status != "processing" {
		t.Errorf("Expected status 'processing', got %s", task.Status)
	}
	if task.LeasedUntil == nil || task.LeasedUntil.Before(time.Now()) {
		t.Errorf("Expected LeasedUntil to be in the future, got %v", task.LeasedUntil)
	}

	// 2. Test leasing a task that timed out
	_, err = db.Exec("UPDATE async_tasks SET status='processing', leased_until=?", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}

	task2, err := w.LeaseNextTask(context.Background())
	if err != nil {
		t.Fatalf("Failed to lease task: %v", err)
	}
	if task2 == nil {
		t.Fatal("Expected timed out task to be leased, got nil")
	}
	if task2.ID != task.ID {
		t.Errorf("Expected to lease the same task ID, got %d", task2.ID)
	}

	// 3. Test retirement after 5 attempts
	_, err = db.Exec("UPDATE async_tasks SET status='pending', attempt_count=5 WHERE id=?", task.ID)
	if err != nil {
		t.Fatalf("Failed to update attempt_count: %v", err)
	}

	task3, err := w.LeaseNextTask(context.Background())
	if err != nil {
		t.Fatalf("Failed to lease task: %v", err)
	}
	if task3 != nil {
		t.Errorf("Expected no task to be leased after 5 attempts, got task ID %d", task3.ID)
	}
}

func TestWorkerExecution(t *testing.T) {
	dbPath := "test_worker_exec.db"
	defer os.Remove(dbPath)

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Setup memory and chunks
	_, err = db.Exec("INSERT INTO memories (id, raw_content, content_hash, status) VALUES (1, 'content', 'hash1', 'active')")
	if err != nil {
		t.Fatalf("Failed to insert memory: %v", err)
	}
	_, err = db.Exec("INSERT INTO memory_chunks (id, memory_id, chunk_index, chunk_text) VALUES (1, 1, 0, 'chunk content')")
	if err != nil {
		t.Fatalf("Failed to insert chunk: %v", err)
	}

	w := worker.NewWorker(db, &mockEmbedder{dim: 768}, &mockGraphExtractor{})

	// Test Embed Task Execution
	_, err = db.Exec("INSERT INTO async_tasks (memory_id, task_type, status) VALUES (1, 'embed', 'pending')")
	if err != nil {
		t.Fatalf("Failed to insert task: %v", err)
	}

	task, err := w.LeaseNextTask(context.Background())
	if err != nil {
		t.Fatalf("Failed to lease task: %v", err)
	}

	err = w.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("Failed to execute embed task: %v", err)
	}

	// Verify task completed
	var status string
	err = db.QueryRow("SELECT status FROM async_tasks WHERE id=?", task.ID).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query task status: %v", err)
	}
	if status != "completed" {
		t.Errorf("Expected status 'completed', got %s", status)
	}

	// Verify vector created (skip if vec0 not working correctly in test environment, but try it)
	var count int
	err = db.QueryRow("SELECT count(*) FROM memory_chunks_vec").Scan(&count)
	if err != nil {
		t.Logf("Vector table query failed (expected if vec0 missing): %v", err)
	} else if count != 1 {
		t.Errorf("Expected 1 vector in memory_chunks_vec, got %d", count)
	}

	// Test Graph Extract Task Execution
	_, err = db.Exec("INSERT INTO async_tasks (memory_id, task_type, status) VALUES (1, 'graph_extract', 'pending')")
	if err != nil {
		t.Fatalf("Failed to insert task: %v", err)
	}

	taskGE, err := w.LeaseNextTask(context.Background())
	if err != nil {
		t.Fatalf("Failed to lease task: %v", err)
	}

	err = w.ExecuteTask(context.Background(), taskGE)
	if err != nil {
		t.Fatalf("Failed to execute graph_extract task: %v", err)
	}

	// Verify nodes and evidence created
	// mockGraphExtractor returns "Entity A", "Entity B"
	err = db.QueryRow("SELECT count(*) FROM kg_nodes").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query kg_nodes: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 nodes, got %d", count)
	}

	err = db.QueryRow("SELECT count(*) FROM kg_edge_evidence").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query kg_edge_evidence: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 edge evidence, got %d", count)
	}
}
