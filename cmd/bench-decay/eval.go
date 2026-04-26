package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hsme/core/src/core/search"
)

type BenchmarkReport struct {
	Config   Config         `json:"config"`
	Queries  []QueryResult  `json:"queries"`
	Exact    []QueryResult  `json:"exact_samples"`
}

type QueryResult struct {
	Query string      `json:"query"`
	Off   []ResultRow `json:"off"`
	On    []ResultRow `json:"on"`
}

type ResultRow struct {
	MemoryID int64   `json:"memory_id"`
	Score    float64 `json:"score"`
}

func runEval(ctx context.Context, db *sql.DB, cfg Config, queries []string) (*BenchmarkReport, error) {
	report := &BenchmarkReport{
		Config: cfg,
	}

	for _, q := range queries {
		// Fuzzy OFF
		search.GlobalDecayConfig = search.DecayConfig{Enabled: false, HalfLifeDays: cfg.HalfLife}
		fuzzyOff, err := search.FuzzySearch(ctx, db, nil, q, 10, "")
		if err != nil {
			return nil, fmt.Errorf("fuzzy off error for %q: %v", q, err)
		}

		// Fuzzy ON
		search.GlobalDecayConfig = search.DecayConfig{Enabled: true, HalfLifeDays: cfg.HalfLife}
		fuzzyOn, err := search.FuzzySearch(ctx, db, nil, q, 10, "")
		if err != nil {
			return nil, fmt.Errorf("fuzzy on error for %q: %v", q, err)
		}

		qRes := QueryResult{Query: q}
		for _, r := range fuzzyOff {
			qRes.Off = append(qRes.Off, ResultRow{MemoryID: r.MemoryID, Score: r.Score})
		}
		for _, r := range fuzzyOn {
			qRes.On = append(qRes.On, ResultRow{MemoryID: r.MemoryID, Score: r.Score})
		}
		report.Queries = append(report.Queries, qRes)

		// Exact OFF
		search.GlobalDecayConfig = search.DecayConfig{Enabled: false, HalfLifeDays: cfg.HalfLife}
		exactOff, err := search.ExactSearch(ctx, db, q, 10, "")
		if err != nil {
			return nil, fmt.Errorf("exact off error for %q: %v", q, err)
		}

		// Exact ON
		search.GlobalDecayConfig = search.DecayConfig{Enabled: true, HalfLifeDays: cfg.HalfLife}
		exactOn, err := search.ExactSearch(ctx, db, q, 10, "")
		if err != nil {
			return nil, fmt.Errorf("exact on error for %q: %v", q, err)
		}

		eRes := QueryResult{Query: q}
		for _, r := range exactOff {
			eRes.Off = append(eRes.Off, ResultRow{MemoryID: r.MemoryID, Score: r.Score})
		}
		for _, r := range exactOn {
			eRes.On = append(eRes.On, ResultRow{MemoryID: r.MemoryID, Score: r.Score})
		}
		report.Exact = append(report.Exact, eRes)
	}

	return report, nil
}
