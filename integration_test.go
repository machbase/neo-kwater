package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/machbase/neo-water/internal/importer"
	"github.com/machbase/neo-water/internal/machbase"
)

func TestIntegrationImportKWDam(t *testing.T) {
	if os.Getenv("KWATER_INTEGRATION") != "1" {
		t.Skip("set KWATER_INTEGRATION=1 to run against local neo-server")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := importer.Config{
		Dir:         "test/data",
		DB:          "127.0.0.1:5656",
		User:        "sys",
		Password:    "manager",
		Table:       "kwdam",
		Concurrency: 4,
	}

	appender, closeAppender, err := machbase.OpenAppender(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenAppender() error = %v", err)
	}
	defer closeAppender()

	result, err := importer.Import(ctx, cfg, appender)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.FilesProcessed != 10 {
		t.Fatalf("FilesProcessed = %d, want 10", result.FilesProcessed)
	}
	if result.RowsAppended != 50 {
		t.Fatalf("RowsAppended = %d, want 50", result.RowsAppended)
	}
	if result.RowsFailed != 0 {
		t.Fatalf("RowsFailed = %d, want 0", result.RowsFailed)
	}
}
