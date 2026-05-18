package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
		printUsageError(stderr, "missing command: import")
		return 2
	}
	if args[0] != "import" {
		if strings.HasPrefix(args[0], "-") {
			printUsageError(stderr, fmt.Sprintf("missing command: import must appear before %s", args[0]))
		} else {
			printUsageError(stderr, fmt.Sprintf("unknown command: %s", args[0]))
		}
		return 2
	}

	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var cfg importer.Config
	flags.StringVar(&cfg.Dir, "dir", "", "directory containing csv files")
	flags.StringVar(&cfg.DB, "db", "", "machbase-neo host:port")
	flags.StringVar(&cfg.User, "user", "sys", "database user")
	flags.StringVar(&cfg.Password, "password", "manager", "database password")
	flags.StringVar(&cfg.Table, "table", "", "target table")
	flags.IntVar(&cfg.Concurrency, "c", 10, "number of files to process concurrently")

	if err := flags.Parse(args[1:]); err != nil {
		printUsage(stderr)
		return 2
	}

	cfg.Progress = importer.NewTerminalProgress(stdout)

	if err := cfg.Validate(); err != nil {
		printUsageError(stderr, err.Error())
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

func printUsageError(w io.Writer, message string) {
	fmt.Fprintf(w, "error: %s\n", message)
	printUsage(w)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: kwater import -dir <dir> -db <host:port> -user <user> -password <password> -table <table> [-c <n>]")
}
