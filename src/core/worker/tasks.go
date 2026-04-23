package worker

import (
	"context"
	"database/sql"
	"time"
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

func (w *Worker) LeaseNextTask(ctx context.Context) (*AsyncTask, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		SELECT id, memory_id, task_type, status, attempt_count, last_error, leased_until
		FROM async_tasks
		WHERE (status = 'pending' OR (status = 'processing' AND leased_until < ?))
		AND attempt_count < 5
		LIMIT 1
	`
	now := time.Now()
	row := tx.QueryRowContext(ctx, query, now.Format(time.RFC3339))

	var task AsyncTask
	var leasedUntilStr sql.NullString
	err = row.Scan(&task.ID, &task.MemoryID, &task.TaskType, &task.Status, &task.AttemptCount, &task.LastError, &leasedUntilStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if leasedUntilStr.Valid {
		t, _ := time.Parse(time.RFC3339, leasedUntilStr.String)
		task.LeasedUntil = &t
	}

	newLeasedUntil := now.Add(30 * time.Second)
	_, err = tx.ExecContext(ctx, "UPDATE async_tasks SET status = 'processing', leased_until = ?, attempt_count = attempt_count + 1 WHERE id = ?", newLeasedUntil.Format(time.RFC3339), task.ID)
	if err != nil {
		return nil, err
	}

	task.Status = "processing"
	task.LeasedUntil = &newLeasedUntil
	task.AttemptCount++

	return &task, tx.Commit()
}
