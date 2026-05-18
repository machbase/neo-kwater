package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/machbase/neo-water/internal/importer"
	"github.com/machbase/neo-water/internal/machbase"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsageError(stderr, "missing command: import or dryrun", "")
		return 2
	}
	switch args[0] {
	case "import":
		return runImport(args[1:], stdout, stderr)
	case "dryrun":
		return runDryRun(args[1:], stdout, stderr)
	default:
		if strings.HasPrefix(args[0], "-") {
			printUsageError(stderr, fmt.Sprintf("missing command: import or dryrun must appear before %s", args[0]), "")
		} else {
			printUsageError(stderr, fmt.Sprintf("unknown command: %s", args[0]), "")
		}
		return 2
	}

}

func runImport(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var cfg importer.Config
	flags.StringVar(&cfg.Dir, "dir", "", "directory containing csv files")
	flags.StringVar(&cfg.DB, "db", "", "machbase-neo host:port")
	flags.StringVar(&cfg.User, "user", "sys", "database user")
	flags.StringVar(&cfg.Password, "password", "manager", "database password")
	flags.StringVar(&cfg.Table, "table", "", "target table")
	flags.IntVar(&cfg.Concurrency, "c", 10, "number of files to process concurrently")
	flags.IntVar(&cfg.IgnoreLowConfidence, "ignore-low-confidence", math.MinInt, "skip records with confidence lower than this value")

	if err := flags.Parse(args); err != nil {
		printUsage(stderr, "import")
		return 2
	}

	cfg.Progress = importer.NewTerminalProgress(stdout)

	if err := cfg.Validate(); err != nil {
		printUsageError(stderr, err.Error(), "import")
		return 2
	}

	appender, closeAppender, err := machbase.OpenAppender(context.Background(), cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closeAppender()

	if _, err := importer.Import(context.Background(), cfg, appender); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func runDryRun(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("dryrun", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var cfg importer.Config
	flags.StringVar(&cfg.Dir, "dir", "", "directory containing csv files")
	flags.IntVar(&cfg.Concurrency, "c", 10, "number of files to process concurrently")
	flags.IntVar(&cfg.IgnoreLowConfidence, "ignore-low-confidence", math.MinInt, "skip records with confidence lower than this value")

	if err := flags.Parse(args); err != nil {
		printUsage(stderr, "dryrun")
		return 2
	}
	cfg.Progress = importer.NewTerminalProgress(stdout)

	if err := cfg.ValidateDryRun(); err != nil {
		printUsageError(stderr, err.Error(), "dryrun")
		return 2
	}

	result, err := importer.DryRun(context.Background(), cfg, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if result.RowsFailed > 0 {
		return 1
	}
	return 0
}

func printUsageError(w io.Writer, message string, command string) {
	fmt.Fprintf(w, "error: %s\n", message)
	printUsage(w, command)
}

func printUsage(w io.Writer, command string) {
	switch command {
	case "import":
		fmt.Fprintln(w, "usage: kwater import -dir <dir> -db <host:port> -user <user> -password <password> -table <table> [-c <n>] [-ignore-low-confidence <n>]")
	case "dryrun":
		fmt.Fprintln(w, "usage: kwater dryrun -dir <dir> [-c <n>] [-ignore-low-confidence <n>]")
	default:
		fmt.Fprintln(w, "usage: kwater import -dir <dir> -db <host:port> -user <user> -password <password> -table <table> [-c <n>] [-ignore-low-confidence <n>]")
		fmt.Fprintln(w, "usage: kwater dryrun -dir <dir> [-c <n>] [-ignore-low-confidence <n>]")
	}
}
