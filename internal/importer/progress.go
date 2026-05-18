package importer

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

type Progress interface {
	Render(Snapshot)
}

type Snapshot struct {
	Files          []FileProgress
	TotalFiles     int
	CompletedFiles int
	RowsAppended   int64
	RowsFailed     int64
}

type FileProgress struct {
	Path       string
	LinesRead  int64
	TotalLines int64
	Done       bool
	Started    bool
}

type progressState struct {
	mu        sync.Mutex
	files     []FileProgress
	completed int
}

func newProgressState(paths []string) *progressState {
	files := make([]FileProgress, len(paths))
	for i, path := range paths {
		files[i] = FileProgress{Path: path}
	}
	return &progressState{files: files}
}

func (s *progressState) start(index int, totalLines int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[index].Started = true
	s.files[index].TotalLines = totalLines
}

func (s *progressState) advance(index int, lines int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[index].LinesRead += lines
}

func (s *progressState) finish(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.files[index].Done {
		s.files[index].Done = true
		s.completed++
	}
}

func (s *progressState) snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	files := make([]FileProgress, len(s.files))
	copy(files, s.files)
	return Snapshot{Files: files, TotalFiles: len(files), CompletedFiles: s.completed}
}

type terminalProgress struct {
	mu sync.Mutex
	w  io.Writer
}

func NewTerminalProgress(w io.Writer) Progress {
	return &terminalProgress{w: w}
}

func (p *terminalProgress) Render(snapshot Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(p.w, "\033[H\033[2J")
	fmt.Fprint(p.w, FormatSnapshot(snapshot))
}

func renderProgress(progress Progress, snapshot Snapshot) {
	if progress != nil {
		progress.Render(snapshot)
	}
}

func FormatSnapshot(snapshot Snapshot) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Total %s of %s files processed.\n", comma(snapshot.CompletedFiles), comma(snapshot.TotalFiles))
	for _, file := range snapshot.Files {
		if !file.Started && !file.Done {
			continue
		}
		fmt.Fprintf(&builder, "%s %s %s\n", file.Path, progressBar(file), fileStatus(file))
	}
	return builder.String()
}

func progressBar(file FileProgress) string {
	const width = 20
	if file.Done {
		return strings.Repeat("#", width)
	}
	if file.TotalLines <= 0 {
		return strings.Repeat(".", width)
	}
	filled := int(file.LinesRead * width / file.TotalLines)
	if filled > width {
		filled = width
	}
	return strings.Repeat("#", filled) + strings.Repeat(".", width-filled)
}

func fileStatus(file FileProgress) string {
	if file.Done {
		return fmt.Sprintf("%s lines Done", comma64(file.LinesRead))
	}
	return fmt.Sprintf("%s/%s lines processing", comma64(file.LinesRead), comma64(file.TotalLines))
}

func comma(value int) string {
	return comma64(int64(value))
}

func comma64(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := fmt.Sprintf("%d", value)
	if len(digits) <= 3 {
		if negative {
			return "-" + digits
		}
		return digits
	}

	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	prefix := len(digits) % 3
	if prefix == 0 {
		prefix = 3
	}
	builder.WriteString(digits[:prefix])
	for i := prefix; i < len(digits); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(digits[i : i+3])
	}
	return builder.String()
}
