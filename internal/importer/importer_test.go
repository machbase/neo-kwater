package importer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeAppender struct {
	mu      sync.Mutex
	records [][]any
	closed  bool
}

func (f *fakeAppender) Append(values ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, append([]any(nil), values...))
	return nil
}

func (f *fakeAppender) Close() (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return int64(len(f.records)), 0, nil
}

type captureProgress struct {
	mu        sync.Mutex
	snapshots []Snapshot
}

func (p *captureProgress) Render(snapshot Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshots = append(p.snapshots, snapshot)
}

func TestImportReadsSortedCSVFilesAndAppendsTypedRows(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "b.csv"), "B,2016-04-28 04:52:00,2.5,90\n")
	mustWrite(t, filepath.Join(dir, "a.csv"), "NAME,TIME,VALUE,CONFIDENCE\nA,2016-04-28 04:51:00,1.25,100\n")
	mustWrite(t, filepath.Join(dir, "ignore.txt"), "X,2016-04-28 04:53:00,3,80\n")

	appender := &fakeAppender{}
	progress := &captureProgress{}
	result, err := Import(context.Background(), Config{
		Dir:         dir,
		DB:          "127.0.0.1:5656",
		User:        "sys",
		Password:    "manager",
		Table:       "EXAMPLE",
		Concurrency: 1,
		Progress:    progress,
	}, appender)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.FilesProcessed != 2 {
		t.Fatalf("FilesProcessed = %d, want 2", result.FilesProcessed)
	}
	if result.RowsAppended != 2 {
		t.Fatalf("RowsAppended = %d, want 2", result.RowsAppended)
	}
	if result.Elapsed <= 0 {
		t.Fatalf("Elapsed = %s, want positive duration", result.Elapsed)
	}
	if !appender.closed {
		t.Fatal("appender was not closed")
	}
	if len(appender.records) != 2 {
		t.Fatalf("records = %d, want 2", len(appender.records))
	}

	assertRecord(t, appender.records[0], "A", "2016-04-28 04:51:00 +0900 KST", 1.25, 100)
	assertRecord(t, appender.records[1], "B", "2016-04-28 04:52:00 +0900 KST", 2.5, 90)

	progress.mu.Lock()
	defer progress.mu.Unlock()
	if len(progress.snapshots) == 0 {
		t.Fatal("expected progress snapshots")
	}
	last := progress.snapshots[len(progress.snapshots)-1]
	if last.CompletedFiles != 2 || last.TotalFiles != 2 {
		t.Fatalf("final progress = %d/%d, want 2/2", last.CompletedFiles, last.TotalFiles)
	}
	if !last.ShowSummary {
		t.Fatal("final progress did not include summary")
	}
}

func TestFormatSnapshotUsesCommasAndProgressBars(t *testing.T) {
	output := FormatSnapshot(Snapshot{
		TotalFiles:     20000,
		CompletedFiles: 2,
		Files: []FileProgress{
			{Path: "./dir/file1.csv", Started: true, Done: true, LinesRead: 12345, TotalLines: 12345},
			{Path: "./dir/file3.csv", Started: true, LinesRead: 34567, TotalLines: 123456},
		},
	})

	wantContains := []string{
		"Total 2 of 20,000 files processed.",
		"./dir/file1.csv #################### 12,345 lines Done",
		"./dir/file3.csv #####............... 34,567/123,456 lines processing",
	}
	for _, want := range wantContains {
		if !stringsContains(output, want) {
			t.Fatalf("FormatSnapshot() missing %q in:\n%s", want, output)
		}
	}
}

func TestProgressTickerRendersSnapshots(t *testing.T) {
	state := newProgressState([]string{"file.csv"})
	state.start(0, 100)
	state.advance(0, 25)

	progress := &captureProgress{}
	ctx, cancel := context.WithCancel(context.Background())
	stop := startProgressTicker(ctx, progress, state, 5*time.Millisecond)
	defer stop()
	defer cancel()

	waitUntil(t, 200*time.Millisecond, func() bool {
		progress.mu.Lock()
		defer progress.mu.Unlock()
		return len(progress.snapshots) > 0
	})

	progress.mu.Lock()
	defer progress.mu.Unlock()
	last := progress.snapshots[len(progress.snapshots)-1]
	if len(last.Files) != 1 {
		t.Fatalf("snapshot files = %d, want 1", len(last.Files))
	}
	if last.Files[0].LinesRead != 25 {
		t.Fatalf("LinesRead = %d, want 25", last.Files[0].LinesRead)
	}
}

func TestFormatSnapshotIncludesFinalSummary(t *testing.T) {
	output := FormatSnapshot(Snapshot{
		TotalFiles:     4,
		CompletedFiles: 4,
		RowsAppended:   1234567,
		RowsFailed:     8,
		Elapsed:        8 * time.Second,
		ShowSummary:    true,
	})

	want := "Summary: 4 files processed, 1,234,567 records succeeded, 8 records failed, elapsed 8s, average 2s per file"
	if !stringsContains(output, want) {
		t.Fatalf("FormatSnapshot() missing summary %q in:\n%s", want, output)
	}
}

func mustWrite(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertRecord(t *testing.T, record []any, name string, timestamp string, value float64, confidence int) {
	t.Helper()
	if got := record[0]; got != name {
		t.Fatalf("name = %v, want %v", got, name)
	}
	if got := record[1].(time.Time).String(); got != timestamp {
		t.Fatalf("time = %v, want %v", got, timestamp)
	}
	if got := record[2]; got != value {
		t.Fatalf("value = %v, want %v", got, value)
	}
	if got := record[3]; got != confidence {
		t.Fatalf("confidence = %v, want %v", got, confidence)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func stringsContains(s string, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && containsAt(s, substr))
}

func containsAt(s string, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
