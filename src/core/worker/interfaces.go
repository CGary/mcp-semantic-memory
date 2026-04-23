package worker

import (
	"context"
	"database/sql"
)

type Embedder interface {
	GenerateVector(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type GraphExtractor interface {
	ExtractEntities(ctx context.Context, text string) ([]string, error)
}

type Worker struct {
	db        *sql.DB
	embedder  Embedder
	extractor GraphExtractor
}

func NewWorker(db *sql.DB, embedder Embedder, extractor GraphExtractor) *Worker {
	return &Worker{
		db:        db,
		embedder:  embedder,
		extractor: extractor,
	}
}
