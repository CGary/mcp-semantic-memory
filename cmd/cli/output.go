package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func ShouldColor() bool {
	return IsTTY() && os.Getenv("NO_COLOR") == "" && !noColorFlag
}

func FormatJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FormatText(v interface{}) string {
	// Simple string representation for text mode
	// Subcommands may override this for more complex structures
	return fmt.Sprintf("%v", v)
}

func WriteResult(w io.Writer, v interface{}, format string) error {
	if format == "json" {
		s, err := FormatJSON(v)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, s)
		return nil
	}
	fmt.Fprintln(w, FormatText(v))
	return nil
}

func WriteError(w io.Writer, err error, code int, format string) {
	if format == "json" {
		res := map[string]interface{}{
			"error": err.Error(),
			"code":  code,
		}
		s, _ := FormatJSON(res)
		fmt.Fprintln(w, s)
		return
	}
	fmt.Fprintf(w, "error: %v\n", err)
}

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
)

func Green(s string) string {
	if !ShouldColor() {
		return s
	}
	return ColorGreen + s + ColorReset
}

func Red(s string) string {
	if !ShouldColor() {
		return s
	}
	return ColorRed + s + ColorReset
}

func Yellow(s string) string {
	if !ShouldColor() {
		return s
	}
	return ColorYellow + s + ColorReset
}
