// Command bench is the Browser Concurrency Research Lab controller CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"bcrl/internal/scenarios"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("bench %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "validate":
		if err := cmdValidate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "list-scenarios":
		cmdListScenarios()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("Usage: bench <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version          print version information")
	fmt.Println("  validate         validate configuration and environment")
	fmt.Println("  list-scenarios   list available benchmark scenarios")
}

func cmdListScenarios() {
	fmt.Printf("%-20s  %s\n", "SCENARIO", "DESCRIPTION")
	for _, s := range scenarios.All() {
		status := "planned"
		if s.Implemented {
			status = "implemented"
		}
		fmt.Printf("%-20s  %s (%s)\n", s.ID, s.Description, status)
	}
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "bench.yaml", "path to configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Config validation is deferred to later phases; for now, report the
	// toolchain this controller is running on.
	fmt.Println("Benchmark environment:")
	fmt.Printf("  OS:        %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Go:        %s\n", runtime.Version())
	fmt.Printf("  Config:    %s\n", *configPath)
	return nil
}
