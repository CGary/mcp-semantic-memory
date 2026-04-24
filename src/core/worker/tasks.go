package worker

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/hsme/core/src/core/indexer"
)

// Enums del extractor — cualquier cosa fuera de esto se descarta.
// Spec §14.4 define estos valores; phi3.5 a veces emite el literal del
// prompt (ej. "TECH|ERROR|FILE|CMD") como si fuera un tipo válido.
var allowedNodeTypes = map[string]struct{}{
	"TECH": {}, "ERROR": {}, "FILE": {}, "CMD": {},
}
var allowedRelationTypes = map[string]struct{}{
	"DEPENDS_ON": {}, "RESOLVES": {}, "CAUSES": {},
}

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
	ModelID() string
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
		chunks, err := w.loadChunks(ctx, task.MemoryID)
		if err != nil {
			return fmt.Errorf("failed to get chunks: %w", err)
		}

		for i := 0; i < len(chunks); i++ {
			chunk := chunks[i]
			vector, err := w.Embedder.GenerateVector(ctx, chunk.Text)
			if err != nil {
				if isEmbeddingContextLengthError(err) {
					rechunked, splitErr := w.rechunkOversizedChunk(ctx, task.MemoryID, chunk.ID, chunk.Index, chunk.Text)
					if splitErr != nil {
						return splitErr
					}
					if rechunked {
						chunks, err = w.loadChunks(ctx, task.MemoryID)
						if err != nil {
							return fmt.Errorf("failed to reload rechunked memory: %w", err)
						}
						i = -1
						continue
					}
				}
				return fmt.Errorf("failed to generate vector: %w", err)
			}

			blob, err := vec.SerializeFloat32(vector)
			if err != nil {
				return fmt.Errorf("failed to serialize vector: %w", err)
			}

			_, err = w.db.ExecContext(ctx, "INSERT OR REPLACE INTO memory_chunks_vec(rowid, embedding) VALUES(?, ?)", chunk.ID, blob)
			if err != nil {
				return fmt.Errorf("failed to insert vector: %w", err)
			}
		}
	} else if task.TaskType == "graph_extract" {
		kg, err := w.GraphExtractor.ExtractEntities(ctx, content)
		if err != nil {
			return fmt.Errorf("failed to extract entities: %w", err)
		}

		// Map original-name → node id para resolver edges. Usamos el nombre CRUDO
		// del extractor como key porque los edges vienen con esos mismos literales.
		nodeIDs := make(map[string]int64)

		for _, node := range kg.Nodes {
			nodeType := indexer.CanonicalizeType(node.Type)
			if _, ok := allowedNodeTypes[nodeType]; !ok {
				// phi3.5 a veces emite basura tipo "TECH|ERROR|FILE|CMD". Descartamos.
				continue
			}
			canonical, display := indexer.CanonicalizeName(node.Name)
			if canonical == "" {
				continue
			}

			var nodeID int64
			err := w.db.QueryRowContext(ctx, `
				INSERT INTO kg_nodes(type, canonical_name, display_name)
				VALUES(?, ?, ?)
				ON CONFLICT(type, canonical_name)
				DO UPDATE SET display_name=excluded.display_name
				RETURNING id`,
				nodeType, canonical, display).Scan(&nodeID)
			if err != nil {
				// Fallback si RETURNING no está disponible por alguna razón
				_, _ = w.db.ExecContext(ctx, "INSERT OR IGNORE INTO kg_nodes(type, canonical_name, display_name) VALUES(?, ?, ?)", nodeType, canonical, display)
				_ = w.db.QueryRowContext(ctx, "SELECT id FROM kg_nodes WHERE type = ? AND canonical_name = ?", nodeType, canonical).Scan(&nodeID)
			}
			nodeIDs[node.Name] = nodeID
		}

		for _, edge := range kg.Edges {
			relation := indexer.CanonicalizeType(edge.Relation)
			if _, ok := allowedRelationTypes[relation]; !ok {
				continue
			}
			sourceID, okS := nodeIDs[edge.Source]
			targetID, okT := nodeIDs[edge.Target]
			if okS && okT {
				_, _ = w.db.ExecContext(ctx, `
					INSERT OR IGNORE INTO kg_edge_evidence(source_node_id, target_node_id, relation_type, memory_id)
					VALUES(?, ?, ?, ?)`,
					sourceID, targetID, relation, task.MemoryID)
			}
		}
	}

	_, err = w.db.ExecContext(ctx, "UPDATE async_tasks SET status = 'completed', completed_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), task.ID)
	return err
}

type chunkRecord struct {
	ID    int64
	Index int
	Text  string
}

func (w *Worker) loadChunks(ctx context.Context, memoryID int64) ([]chunkRecord, error) {
	rows, err := w.db.QueryContext(ctx, "SELECT id, chunk_index, chunk_text FROM memory_chunks WHERE memory_id = ? ORDER BY chunk_index", memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []chunkRecord
	for rows.Next() {
		var chunk chunkRecord
		if err := rows.Scan(&chunk.ID, &chunk.Index, &chunk.Text); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func isEmbeddingContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "input length exceeds the context length")
}

func (w *Worker) rechunkOversizedChunk(ctx context.Context, memoryID, chunkID int64, chunkIndex int, chunkText string) (bool, error) {
	subChunks := indexer.Split(chunkText, "note")
	if len(subChunks) <= 1 {
		return false, nil
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("failed to begin rechunk tx: %w", err)
	}
	defer tx.Rollback()

	shiftBy := len(subChunks) - 1
	if shiftBy > 0 {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, chunk_index
			  FROM memory_chunks
			 WHERE memory_id = ?
			   AND chunk_index > ?
			 ORDER BY chunk_index DESC`, memoryID, chunkIndex)
		if err != nil {
			return false, fmt.Errorf("failed to load following chunks for rechunk: %w", err)
		}
		type followingChunk struct {
			id    int64
			index int
		}
		var following []followingChunk
		for rows.Next() {
			var fc followingChunk
			if err := rows.Scan(&fc.id, &fc.index); err != nil {
				rows.Close()
				return false, fmt.Errorf("failed to scan following chunk: %w", err)
			}
			following = append(following, fc)
		}
		rows.Close()

		for _, fc := range following {
			if _, err := tx.ExecContext(ctx, `
				UPDATE memory_chunks
				   SET chunk_index = ?
				 WHERE id = ?`, fc.index+shiftBy, fc.id); err != nil {
				return false, fmt.Errorf("failed to shift following chunk: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_chunks
		   SET chunk_text = ?, token_estimate = ?
		 WHERE id = ?`,
		subChunks[0], estimateTokens(subChunks[0]), chunkID); err != nil {
		return false, fmt.Errorf("failed to rewrite oversized chunk: %w", err)
	}

	for idx := 1; idx < len(subChunks); idx++ {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO memory_chunks(memory_id, chunk_index, chunk_text, token_estimate)
			VALUES(?, ?, ?, ?)`,
			memoryID, chunkIndex+idx, subChunks[idx], estimateTokens(subChunks[idx])); err != nil {
			return false, fmt.Errorf("failed to insert rechunked tail: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit rechunk tx: %w", err)
	}
	return true, nil
}

func estimateTokens(text string) int {
	return len(strings.Fields(text))
}
