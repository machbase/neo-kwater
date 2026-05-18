package importer

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type DryRunIssue struct {
	fileIndex int
	Path      string
	Line      int
	Content   string
	Err       error
}

type dryRunCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
	issues  []DryRunIssue
}

func DryRun(ctx context.Context, cfg Config, issueWriter io.Writer) (Result, error) {
	startedAt := time.Now()

	if err := cfg.ValidateDryRun(); err != nil {
		return Result{}, err
	}

	files, err := csvFiles(cfg.Dir)
	if err != nil {
		return Result{}, err
	}

	state := newProgressState(files)
	counters := &dryRunCounters{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopProgress := startProgressTicker(ctx, cfg.Progress, state, progressRenderInterval)
	defer stopProgress()
	renderProgress(cfg.Progress, state.snapshot())

	jobs := make(chan fileJob)
	errs := make(chan error, 1)
	var workers sync.WaitGroup

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return Result{}, err
	}

	workerCount := min(cfg.Concurrency, max(1, len(files)))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				if err := dryRunFile(ctx, job, loc, state, cfg.Progress, cfg.IgnoreLowConfidence, counters); err != nil {
					select {
					case errs <- err:
					default:
					}
					cancel()
				}
			}
		}()
	}

dispatch:
	for index, path := range files {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- fileJob{index: index, path: path}:
		}
		if ctx.Err() != nil {
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	stopProgress()

	select {
	case err := <-errs:
		return Result{}, err
	default:
	}

	success, failure, issues := counters.snapshot()
	elapsed := time.Since(startedAt)
	finalSnapshot := state.snapshot()
	finalSnapshot.RowsAppended = success
	finalSnapshot.RowsFailed = failure
	finalSnapshot.Elapsed = elapsed
	finalSnapshot.ShowSummary = true
	renderProgress(cfg.Progress, finalSnapshot)
	writeDryRunIssues(issueWriter, issues)

	return Result{FilesProcessed: finalSnapshot.CompletedFiles, RowsAppended: success, RowsFailed: failure, Elapsed: elapsed}, nil
}

func dryRunFile(ctx context.Context, job fileJob, loc *time.Location, state *progressState, progress Progress, ignoreLowConfidence int, counters *dryRunCounters) error {
	totalLines, err := countLines(job.path)
	if err != nil {
		return fmt.Errorf("count %s: %w", job.path, err)
	}
	state.start(job.index, totalLines)
	renderProgress(progress, state.snapshot())

	file, err := os.Open(job.path)
	if err != nil {
		return fmt.Errorf("open %s: %w", job.path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line++
		content := scanner.Text()
		row, err := parseCSVLine(content)
		if err != nil {
			counters.addIssue(DryRunIssue{fileIndex: job.index, Path: job.path, Line: line, Content: content, Err: err})
			state.advance(job.index, 1)
			continue
		}
		if line == 1 && isHeader(row) {
			state.advance(job.index, 1)
			continue
		}

		_, skip, err := parseRecord(row, loc, ignoreLowConfidence)
		if err != nil {
			counters.addIssue(DryRunIssue{fileIndex: job.index, Path: job.path, Line: line, Content: content, Err: err})
			state.advance(job.index, 1)
			continue
		}
		if !skip {
			counters.addSuccess()
		}
		state.advance(job.index, 1)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", job.path, err)
	}
	state.finish(job.index)
	renderProgress(progress, state.snapshot())
	return nil
}

func parseCSVLine(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.FieldsPerRecord = -1
	row, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (c *dryRunCounters) addSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.success++
}

func (c *dryRunCounters) addIssue(issue DryRunIssue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failure++
	c.issues = append(c.issues, issue)
}

func (c *dryRunCounters) snapshot() (int64, int64, []DryRunIssue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	issues := make([]DryRunIssue, len(c.issues))
	copy(issues, c.issues)
	sort.Slice(issues, func(i int, j int) bool {
		if issues[i].fileIndex != issues[j].fileIndex {
			return issues[i].fileIndex < issues[j].fileIndex
		}
		return issues[i].Line < issues[j].Line
	})
	return c.success, c.failure, issues
}

func writeDryRunIssues(w io.Writer, issues []DryRunIssue) {
	if w == nil || len(issues) == 0 {
		return
	}
	fmt.Fprintln(w, "Invalid records:")
	for _, issue := range issues {
		fmt.Fprintf(w, "%s:%d: %s (%v)\n", issue.Path, issue.Line, issue.Content, issue.Err)
	}
}
