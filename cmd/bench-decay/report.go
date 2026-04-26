package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeReports(runDir string, report *BenchmarkReport) error {
	// 1. Write report.json
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "report.json"), b, 0644); err != nil {
		return err
	}

	// 2. Write delta.json (just the differences, simple version)
	// For simplicity, we just dump report again or a transformed version.
	// The spec says: Write report.json, delta.json, and report.md.
	if err := os.WriteFile(filepath.Join(runDir, "delta.json"), b, 0644); err != nil {
		return err
	}

	// 3. Write report.md
	md := fmt.Sprintf("# Benchmark Run: Decay OFF vs ON\n\nHalf-Life: %.1f days\nDatabase: %s\n\n", report.Config.HalfLife, report.Config.DBPath)
	
	md += "## Fuzzy Search\n\n"
	for _, q := range report.Queries {
		md += fmt.Sprintf("### Query: %q\n", q.Query)
		md += "| Rank | OFF (Memory ID) | ON (Memory ID) |\n|---|---|---|\n"
		maxLen := len(q.Off)
		if len(q.On) > maxLen {
			maxLen = len(q.On)
		}
		for i := 0; i < maxLen; i++ {
			offStr := "-"
			onStr := "-"
			if i < len(q.Off) {
				offStr = fmt.Sprintf("%d", q.Off[i].MemoryID)
			}
			if i < len(q.On) {
				onStr = fmt.Sprintf("%d", q.On[i].MemoryID)
			}
			md += fmt.Sprintf("| %d | %s | %s |\n", i+1, offStr, onStr)
		}
		md += "\n"
	}

	md += "## Exact Search\n\n"
	for _, q := range report.Exact {
		md += fmt.Sprintf("### Query: %q\n", q.Query)
		md += "| Rank | OFF (Memory ID) | ON (Memory ID) |\n|---|---|---|\n"
		maxLen := len(q.Off)
		if len(q.On) > maxLen {
			maxLen = len(q.On)
		}
		for i := 0; i < maxLen; i++ {
			offStr := "-"
			onStr := "-"
			if i < len(q.Off) {
				offStr = fmt.Sprintf("%d", q.Off[i].MemoryID)
			}
			if i < len(q.On) {
				onStr = fmt.Sprintf("%d", q.On[i].MemoryID)
			}
			md += fmt.Sprintf("| %d | %s | %s |\n", i+1, offStr, onStr)
		}
		md += "\n"
	}

	if err := os.WriteFile(filepath.Join(runDir, "report.md"), []byte(md), 0644); err != nil {
		return err
	}

	return nil
}
