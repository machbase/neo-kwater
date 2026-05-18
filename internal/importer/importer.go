package importer

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const csvTimeLayout = "2006-01-02 15:04:05"

type Config struct {
	Dir                 string
	DB                  string
	User                string
	Password            string
	Table               string
	Concurrency         int
	IgnoreLowConfidence int
	Progress            Progress
}

func (c Config) Validate() error {
	if c.Dir == "" {
		return errors.New("-dir is required")
	}
	if c.DB == "" {
		return errors.New("-db is required")
	}
	if c.User == "" {
		return errors.New("-user is required")
	}
	if c.Table == "" {
		return errors.New("-table is required")
	}
	if c.Concurrency <= 0 {
		return errors.New("-c must be greater than 0")
	}
	return nil
}

func (c Config) ValidateDryRun() error {
	if c.Dir == "" {
		return errors.New("-dir is required")
	}
	if c.Concurrency <= 0 {
		return errors.New("-c must be greater than 0")
	}
	return nil
}

type Appender interface {
	Append(values ...any) error
	Close() (success int64, fail int64, err error)
}

type Result struct {
	FilesProcessed int
	RowsAppended   int64
	RowsFailed     int64
	Elapsed        time.Duration
}

type record struct {
	values []any
}

type fileJob struct {
	index int
	path  string
}

func Import(ctx context.Context, cfg Config, appender Appender) (Result, error) {
	startedAt := time.Now()

	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	if appender == nil {
		return Result{}, errors.New("appender is required")
	}

	files, err := csvFiles(cfg.Dir)
	if err != nil {
		return Result{}, err
	}

	state := newProgressState(files)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopProgress := startProgressTicker(ctx, cfg.Progress, state, progressRenderInterval)
	defer stopProgress()
	renderProgress(cfg.Progress, state.snapshot())

	jobs := make(chan fileJob)
	records := make(chan record, cfg.Concurrency*128)
	errs := make(chan error, 1)
	var workers sync.WaitGroup
	var appendErr error
	var appendDone sync.WaitGroup

	appendDone.Add(1)
	go func() {
		defer appendDone.Done()
		for rec := range records {
			if err := appender.Append(rec.values...); err != nil {
				appendErr = err
				cancel()
				return
			}
		}
	}()

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
				if err := processFile(ctx, job, loc, records, state, cfg.Progress, cfg.IgnoreLowConfidence); err != nil {
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
	close(records)
	appendDone.Wait()
	stopProgress()

	success, fail, closeErr := appender.Close()
	elapsed := time.Since(startedAt)
	finalSnapshot := state.snapshot()
	finalSnapshot.RowsAppended = success
	finalSnapshot.RowsFailed = fail
	finalSnapshot.Elapsed = elapsed
	finalSnapshot.ShowSummary = true
	renderProgress(cfg.Progress, finalSnapshot)

	if appendErr != nil {
		return Result{}, appendErr
	}
	select {
	case err := <-errs:
		return Result{}, err
	default:
	}
	if closeErr != nil {
		return Result{}, closeErr
	}

	return Result{FilesProcessed: finalSnapshot.CompletedFiles, RowsAppended: success, RowsFailed: fail, Elapsed: elapsed}, nil
}

func csvFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".csv") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func processFile(ctx context.Context, job fileJob, loc *time.Location, records chan<- record, state *progressState, progress Progress, ignoreLowConfidence int) error {
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

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	line := 0
	for {
		raw, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s line %d: %w", job.path, line+1, err)
		}
		line++
		if line == 1 && isHeader(raw) {
			state.advance(job.index, 1)
			continue
		}

		values, skip, err := parseRecord(raw, loc, ignoreLowConfidence)
		if err != nil {
			return fmt.Errorf("parse %s line %d: %w", job.path, line, err)
		}
		if skip {
			state.advance(job.index, 1)
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case records <- record{values: values}:
			state.advance(job.index, 1)
		}
	}
	state.finish(job.index)
	renderProgress(progress, state.snapshot())
	return nil
}

func parseRecord(row []string, loc *time.Location, ignoreLowConfidence int) ([]any, bool, error) {
	if len(row) != 3 && len(row) != 4 {
		return nil, false, fmt.Errorf("expected 3 or 4 fields, got %d", len(row))
	}

	name := strings.TrimSpace(row[0])
	if name == "" {
		return nil, false, errors.New("name is empty")
	}

	timestamp, err := time.ParseInLocation(csvTimeLayout, strings.TrimSpace(row[1]), loc)
	if err != nil {
		return nil, false, err
	}

	valueText := ""
	confidenceText := ""
	if len(row) == 3 {
		confidenceText = strings.TrimSpace(row[2])
	} else {
		valueText = strings.TrimSpace(row[2])
		confidenceText = strings.TrimSpace(row[3])
	}

	confidence, err := strconv.Atoi(confidenceText)
	if err != nil {
		return nil, false, err
	}
	if confidence <= ignoreLowConfidence {
		return nil, true, nil
	}

	var value any
	if valueText == "" {
		value = nil
	} else {
		parsed, err := strconv.ParseFloat(valueText, 64)
		if err != nil {
			value = nil
		} else {
			value = parsed
		}
	}

	return []any{name, timestamp, value, confidence}, false, nil
}

func countLines(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lines int64
	for scanner.Scan() {
		lines++
	}
	return lines, scanner.Err()
}

func isHeader(row []string) bool {
	if len(row) == 3 {
		return strings.EqualFold(strings.TrimSpace(row[0]), "NAME") &&
			strings.EqualFold(strings.TrimSpace(row[1]), "TIME") &&
			strings.EqualFold(strings.TrimSpace(row[2]), "CONFIDENCE")
	}
	if len(row) == 4 {
		return strings.EqualFold(strings.TrimSpace(row[0]), "NAME") &&
			strings.EqualFold(strings.TrimSpace(row[1]), "TIME") &&
			strings.EqualFold(strings.TrimSpace(row[2]), "VALUE") &&
			strings.EqualFold(strings.TrimSpace(row[3]), "CONFIDENCE")
	}
	return false
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
