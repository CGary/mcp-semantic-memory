package worker

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

type AsyncTask struct {
	ID           int64
	MemoryID     int64
	TaskType     string
	Status       string
	AttemptCount int
	LastError    *string
	LeasedUntil  *time.Time
}

type Embedder interface {
	GenerateVector(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type Node struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type Edge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

type KnowledgeGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type GraphExtractor interface {
	ExtractEntities(ctx context.Context, text string) (KnowledgeGraph, error)
}

type Worker struct {
	db             *sql.DB
	Embedder       Embedder
	GraphExtractor GraphExtractor
}

func NewWorker(db *sql.DB, embedder Embedder, extractor GraphExtractor) *Worker {
	return &Worker{
		db:             db,
		Embedder:       embedder,
		GraphExtractor: extractor,
	}
}

func (w *Worker) LeaseNextTask(ctx context.Context) (*AsyncTask, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		UPDATE async_tasks
		SET status = 'processing',
		    leased_until = ?,
		    attempt_count = attempt_count + 1,
		    updated_at = ?
		WHERE id = (
			SELECT id FROM async_tasks
			WHERE (status = 'pending' OR (status = 'processing' AND leased_until < ?))
			AND attempt_count < 5
			ORDER BY created_at
			LIMIT 1
		)
		RETURNING id, memory_id, task_type, status, attempt_count, last_error, leased_until
	`
	now := time.Now()
	leaseDuration := 5 * time.Minute
	leasedUntil := now.Add(leaseDuration)

	var task AsyncTask
	var leasedUntilStr string
	err = tx.QueryRowContext(ctx, query, leasedUntil.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339)).Scan(
		&task.ID, &task.MemoryID, &task.TaskType, &task.Status, &task.AttemptCount, &task.LastError, &leasedUntilStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	t, _ := time.Parse(time.RFC3339, leasedUntilStr)
	task.LeasedUntil = &t

	return &task, tx.Commit()
}

func (w *Worker) ExecuteTask(ctx context.Context, task *AsyncTask) error {
	var content string
	err := w.db.QueryRowContext(ctx, "SELECT raw_content FROM memories WHERE id = ?", task.MemoryID).Scan(&content)
	if err != nil {
		return fmt.Errorf("failed to get memory content: %w", err)
	}

	if task.TaskType == "embed" {
		rows, err := w.db.QueryContext(ctx, "SELECT id, chunk_text FROM memory_chunks WHERE memory_id = ?", task.MemoryID)
		if err != nil {
			return fmt.Errorf("failed to get chunks: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var chunkID int64
			var chunkText string
			if err := rows.Scan(&chunkID, &chunkText); err != nil {
				return err
			}

			vector, err := w.Embedder.GenerateVector(ctx, chunkText)
			if err != nil {
				return fmt.Errorf("failed to generate vector: %w", err)
			}

			blob, err := vec.SerializeFloat32(vector)
			if err != nil {
				return fmt.Errorf("failed to serialize vector: %w", err)
			}

			_, err = w.db.ExecContext(ctx, "INSERT OR REPLACE INTO memory_chunks_vec(rowid, embedding) VALUES(?, ?)", chunkID, blob)
			if err != nil {
				return fmt.Errorf("failed to insert vector: %w", err)
			}
		}
	} else if task.TaskType == "graph_extract" {
		kg, err := w.GraphExtractor.ExtractEntities(ctx, content)
		if err != nil {
			return fmt.Errorf("failed to extract entities: %w", err)
		}

		// Map to store name -> id for resolving edges
		nodeIDs := make(map[string]int64)

		for _, node := range kg.Nodes {
			var nodeID int64
			err := w.db.QueryRowContext(ctx, `
				INSERT INTO kg_nodes(type, canonical_name, display_name) 
				VALUES(?, ?, ?) 
				ON CONFLICT(type, canonical_name) 
				DO UPDATE SET display_name=excluded.display_name 
				RETURNING id`,
				node.Type, node.Name, node.Name).Scan(&nodeID)
			if err != nil {
				// Fallback if RETURNING is not supported or other error
				_, _ = w.db.ExecContext(ctx, "INSERT OR IGNORE INTO kg_nodes(type, canonical_name, display_name) VALUES(?, ?, ?)", node.Type, node.Name, node.Name)
				_ = w.db.QueryRowContext(ctx, "SELECT id FROM kg_nodes WHERE type = ? AND canonical_name = ?", node.Type, node.Name).Scan(&nodeID)
			}
			nodeIDs[node.Name] = nodeID
		}

		for _, edge := range kg.Edges {
			sourceID, okS := nodeIDs[edge.Source]
			targetID, okT := nodeIDs[edge.Target]
			if okS && okT {
				_, _ = w.db.ExecContext(ctx, `
					INSERT OR IGNORE INTO kg_edge_evidence(source_node_id, target_node_id, relation_type, memory_id) 
					VALUES(?, ?, ?, ?)`,
					sourceID, targetID, edge.Relation, task.MemoryID)
			}
		}
	}

	_, err = w.db.ExecContext(ctx, "UPDATE async_tasks SET status = 'completed', completed_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), task.ID)
	return err
}
