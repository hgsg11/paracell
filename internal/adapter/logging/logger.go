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
	path := dailyPath(l.path, entry.Time)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	line := entry.String() + "\n"
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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

func dailyPath(path string, now time.Time) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	return filepath.Join(filepath.Dir(path), base+"-"+now.Format("20060102")+ext)
}
