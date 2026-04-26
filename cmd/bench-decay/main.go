package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	DBPath     string
	HalfLife   float64
	OutputDir  string
	Queries    string
}

func main() {
	vec.Auto()
	var cfg Config
	flag.StringVar(&cfg.DBPath, "db", "data/engram.db", "Path to SQLite database")
	flag.Float64Var(&cfg.HalfLife, "half-life", 14.0, "Half-life in days for decay")
	flag.StringVar(&cfg.OutputDir, "out", "data/benchmarks", "Output directory for reports")
	flag.StringVar(&cfg.Queries, "queries", "collision,timeout,api,agent,spec", "Comma-separated queries to benchmark")
	flag.Parse()

	if cfg.HalfLife <= 0 {
		fmt.Fprintf(os.Stderr, "Error: half-life must be > 0\n")
		os.Exit(1)
	}

	// Open DB read-only
	dsn := fmt.Sprintf("file:%s?mode=ro", cfg.DBPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error pinging database: %v\n", err)
	}

	runID := time.Now().Format("20060102T150405Z")
	runDir := filepath.Join(cfg.OutputDir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	queryList := strings.Split(cfg.Queries, ",")
	
	fmt.Printf("Starting benchmark run %s\n", runID)
	
	report, err := runEval(ctx, db, cfg, queryList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running evaluation: %v\n", err)
		os.Exit(1)
	}

	if err := writeReports(runDir, report); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing reports: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Benchmark complete. Reports saved to %s\n", runDir)
}
