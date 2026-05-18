package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunReportsMissingImportCommandBeforeFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"-dir", "./test/data", "-db", "127.0.0.1:5656", "-table", "kwdam", "-c", "2"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	assertContains(t, stderr.String(), "error: missing command: import or dryrun must appear before -dir")
	assertContains(t, stderr.String(), "usage: kwater import")
	assertContains(t, stderr.String(), "usage: kwater dryrun")
}

func TestRunReportsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"load", "-dir", "./test/data"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	assertContains(t, stderr.String(), "error: unknown command: load")
	assertContains(t, stderr.String(), "usage: kwater import")
	assertContains(t, stderr.String(), "usage: kwater dryrun")
}

func TestRunReportsMissingRequiredFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"import", "-dir", "./test/data", "-db", "127.0.0.1:5656"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	assertContains(t, stderr.String(), "error: -table is required")
	assertContains(t, stderr.String(), "usage: kwater import")
}

func TestRunReportsDryRunMissingRequiredFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runWithIO([]string{"dryrun"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	assertContains(t, stderr.String(), "error: -dir is required")
	assertContains(t, stderr.String(), "usage: kwater dryrun")
}

func assertContains(t *testing.T, text string, substr string) {
	t.Helper()
	if !strings.Contains(text, substr) {
		t.Fatalf("expected %q to contain %q", text, substr)
	}
}
