package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelError Level = "ERROR"

	MaxFileSize int64 = 5 * 1024 * 1024
)

type Entry struct {
	Time    time.Time
	Level   Level
	Source  string
	Content string
}

func (e Entry) String() string {
	return fmt.Sprintf("%s %-5s [%s] %s", e.Time.Format("2006-01-02 15:04:05.000"), e.Level, e.Source, e.Content)
}

type Logger struct {
	path    string
	now     func() time.Time
	mu      sync.Mutex
	entries chan Entry
}

func New(path string) *Logger {
	return &Logger{
		path:    path,
		now:     time.Now,
		entries: make(chan Entry, 1024),
	}
}

func NewFile(path string) *Logger {
	return &Logger{
		path: path,
		now:  time.Now,
	}
}

func (l *Logger) Entries() <-chan Entry {
	return l.entries
}

func (l *Logger) Write(level Level, source string, content string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "paracell"
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range lines {
		entry := Entry{
			Time:    now,
			Level:   level,
			Source:  source,
			Content: strings.TrimSuffix(line, "\r"),
		}
		if err := l.append(entry); err != nil {
			return err
		}
		if l.entries != nil {
			l.entries <- entry
		}
	}
	return nil
}

func (l *Logger) append(entry Entry) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	line := entry.String() + "\n"
	if err := l.rotateIfNeeded(int64(len([]byte(line))), entry.Time); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if _, err := file.WriteString(line); err != nil {
		_ = file.Close()
		return fmt.Errorf("write log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

func (l *Logger) rotateIfNeeded(nextSize int64, now time.Time) error {
	info, err := os.Stat(l.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat log file: %w", err)
	}
	if info.Size() == 0 || info.Size()+nextSize <= MaxFileSize {
		return nil
	}
	rotated, err := availableRotatedPath(filepath.Dir(l.path), now)
	if err != nil {
		return err
	}
	if err := os.Rename(l.path, rotated); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return nil
}

func availableRotatedPath(dir string, now time.Time) (string, error) {
	base := "paracell-" + now.Format("20060102-150405.000000000")
	for generation := 0; ; generation++ {
		name := base + ".log"
		if generation > 0 {
			name = fmt.Sprintf("%s-%d.log", base, generation)
		}
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return path, nil
		}
		if err != nil {
			return "", fmt.Errorf("stat rotated log file: %w", err)
		}
	}
}
