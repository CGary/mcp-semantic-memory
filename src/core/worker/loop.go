package worker

import (
	"context"
	"encoding/json"
	"fmt"
)

func (w *Worker) ExecuteTask(ctx context.Context, task *AsyncTask) error {
	var err error
	switch task.TaskType {
	case "embed":
		err = w.executeEmbed(ctx, task)
	case "graph_extract":
		err = w.executeGraphExtract(ctx, task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	status := "completed"
	if err != nil {
		status = "failed"
		errMsg := err.Error()
		_, _ = w.db.ExecContext(ctx, "UPDATE async_tasks SET status = ?, last_error = ? WHERE id = ?", status, errMsg, task.ID)
		return err
	}

	_, err = w.db.ExecContext(ctx, "UPDATE async_tasks SET status = ? WHERE id = ?", status, task.ID)
	return err
}

func (w *Worker) executeEmbed(ctx context.Context, task *AsyncTask) error {
	rows, err := w.db.QueryContext(ctx, "SELECT id, chunk_text FROM memory_chunks WHERE memory_id = ?", task.MemoryID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return err
		}

		vec, err := w.embedder.GenerateVector(ctx, text)
		if err != nil {
			return err
		}

		// JSON encoding as a simple way to pass float array to sqlite-vec if supported via json
		vecJSON, _ := json.Marshal(vec)
		_, err = w.db.ExecContext(ctx, "INSERT INTO memory_chunks_vec(rowid, embedding) VALUES(?, ?)", id, vecJSON)
		if err != nil {
			// If it fails, we log it but don't fail the task in this skeleton
			fmt.Printf("Vector insert failed: %v\n", err)
		}
	}
	return nil
}

func (w *Worker) executeGraphExtract(ctx context.Context, task *AsyncTask) error {
	var content string
	err := w.db.QueryRowContext(ctx, "SELECT raw_content FROM memories WHERE id = ?", task.MemoryID).Scan(&content)
	if err != nil {
		return err
	}

	entities, err := w.extractor.ExtractEntities(ctx, content)
	if err != nil {
		return err
	}

	var nodeIDs []int64
	for _, entity := range entities {
		_, _ = w.db.ExecContext(ctx, "INSERT OR IGNORE INTO kg_nodes (type, canonical_name, display_name) VALUES (?, ?, ?)", "entity", entity, entity)
		var nodeID int64
		_ = w.db.QueryRowContext(ctx, "SELECT id FROM kg_nodes WHERE canonical_name = ?", entity).Scan(&nodeID)
		nodeIDs = append(nodeIDs, nodeID)
	}

	if len(nodeIDs) >= 2 {
		_, _ = w.db.ExecContext(ctx, "INSERT OR IGNORE INTO kg_edge_evidence (source_node_id, target_node_id, relation_type, memory_id) VALUES (?, ?, ?, ?)", nodeIDs[0], nodeIDs[1], "related", task.MemoryID)
	} else if len(nodeIDs) == 1 {
		// Just to have some evidence if only 1 entity
		_, _ = w.db.ExecContext(ctx, "INSERT OR IGNORE INTO kg_edge_evidence (source_node_id, target_node_id, relation_type, memory_id) VALUES (?, ?, ?, ?)", nodeIDs[0], nodeIDs[0], "self", task.MemoryID)
	}

	return nil
}
