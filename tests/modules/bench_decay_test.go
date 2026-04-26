package modules

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBenchDecayCLI(t *testing.T) {
	// Smoke test to ensure the bench-decay tool compiles and runs
	
	// Create a temp directory for the test
	tempDir, err := os.MkdirTemp("", "bench-decay-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Build the CLI
	cmdPath := filepath.Join(tempDir, "bench-decay")
	buildCmd := exec.Command("go", "build", "-tags", "sqlite_fts5 sqlite_vec", "-o", cmdPath, "../../cmd/bench-decay")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build bench-decay: %v\nOutput: %s", err, string(out))
	}

	// Create a dummy SQLite DB
	// We actually need an initialized DB, but the CLI doesn't init.
	// We'll skip DB init here and let it fail gracefully or just test invalid flags

	t.Run("InvalidHalfLife", func(t *testing.T) {
		cmd := exec.Command(cmdPath, "-half-life", "-5")
		if err := cmd.Run(); err == nil {
			t.Errorf("expected bench-decay to fail with invalid half-life")
		}
	})

	t.Run("BasicRun", func(t *testing.T) {
		// Since we don't have a fully populated DB, we just ensure it handles missing DB properly
		// or creates the report directory if it runs
		outDir := filepath.Join(tempDir, "reports")
		cmd := exec.Command(cmdPath, "-db", "nonexistent.db", "-out", outDir)
		err := cmd.Run()
		if err == nil {
			t.Errorf("expected failure on nonexistent db")
		}
	})
}
