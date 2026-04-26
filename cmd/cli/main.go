//go:build sqlite_fts5 && sqlite_vec

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hsme/core/src/bootstrap"
)

var (
	exitUsage   = 1
	exitRuntime = 2
)

func main() {
	// 1. Load defaults from env
	cfg := bootstrap.LoadFromEnv()

	// 2. Register global flags on flag.CommandLine
	RegisterDBFlags(flag.CommandLine, &cfg)

	// 3. Set custom usage
	flag.Usage = printTopLevelHelp

	// 4. Parse global flags (stops at first non-flag arg)
	flag.Parse()

	// 5. Remaining args are the subcommand and its flags
	args := flag.Args()
	if len(args) < 1 {
		// Check for --help or -h specifically if no subcommand
		// flag.Parse() handles --help if it sees it, but only if it's the first thing?
		// Actually flag.Parse() will call flag.Usage() and exit if it sees -h or --help.
		printTopLevelHelp()
		os.Exit(exitUsage)
	}

	subcommand := args[0]
	subArgs := args[1:]

	// Re-check for help as subcommand
	if subcommand == "help" {
		runHelp(subArgs)
		os.Exit(0)
	}

	// Dispatch
	switch subcommand {
	case "store":
		runStore(subArgs, cfg)
	case "search-fuzzy":
		runSearchFuzzy(subArgs, cfg)
	case "search-exact":
		runSearchExact(subArgs, cfg)
	case "explore":
		runExplore(subArgs, cfg)
	case "status":
		runStatus(subArgs, cfg)
	case "admin":
		runAdmin(subArgs, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", subcommand)
		printTopLevelHelp()
		os.Exit(exitUsage)
	}
}
