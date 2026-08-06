package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerは時刻レベル処理元内容を保存して配信する(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".paracell", "logs", "paracell.log")
	logger := New(path)
	logger.now = func() time.Time {
		return time.Date(2026, time.July, 27, 12, 34, 56, 789000000, time.Local)
	}
	daily := filepath.Join(dir, ".paracell", "logs", "paracell-20260727.log")

	if err := logger.Write(LevelInfo, "git", "stdout: created worktree"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(daily)
	if err != nil {
		t.Fatalf("read log failed: %v", err)
	}
	want := "2026-07-27 12:34:56.789 INFO  [git] stdout: created worktree\n"
	if string(data) != want {
		t.Fatalf("log = %q, want %q", data, want)
	}
	select {
	case entry := <-logger.Entries():
		if entry.String() != strings.TrimSuffix(want, "\n") {
			t.Fatalf("entry = %q, want %q", entry.String(), strings.TrimSuffix(want, "\n"))
		}
	default:
		t.Fatal("entry was not delivered")
	}
}

func TestLoggerは日付単位のログへ追記して前日のログを残す(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "paracell.log")
	logger := New(path)
	now := time.Date(2026, time.July, 27, 23, 59, 59, 0, time.Local)
	logger.now = func() time.Time { return now }

	if err := logger.Write(LevelInfo, "paracell", "first"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := logger.Write(LevelInfo, "paracell", "second"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	now = now.Add(time.Second)
	if err := logger.Write(LevelError, "paracell", "next day"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	previous, err := os.ReadFile(filepath.Join(dir, "logs", "paracell-20260727.log"))
	if err != nil {
		t.Fatalf("previous daily log not found: %v", err)
	}
	if !strings.Contains(string(previous), "[paracell] first") || !strings.Contains(string(previous), "[paracell] second") {
		t.Fatalf("previous daily log = %q", previous)
	}
	next, err := os.ReadFile(filepath.Join(dir, "logs", "paracell-20260728.log"))
	if err != nil {
		t.Fatalf("next daily log not found: %v", err)
	}
	if !strings.Contains(string(next), "ERROR [paracell] next day") {
		t.Fatalf("next daily log = %q", next)
	}
}

func TestLoggerは複数行の各行へ時刻レベル処理元を付けて破棄しない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "paracell.log")
	logger := New(path)
	logger.now = func() time.Time { return time.Date(2026, 7, 27, 0, 0, 0, 0, time.Local) }

	if err := logger.Write(LevelError, "paracell", "first line\nsecond line"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "paracell-20260727.log"))
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-07-27 00:00:00.000 ERROR [paracell] first line\n" +
		"2026-07-27 00:00:00.000 ERROR [paracell] second line\n"
	if string(data) != want {
		t.Fatalf("multiline content was discarded: %q", data)
	}
	if (<-logger.Entries()).Content != "first line" || (<-logger.Entries()).Content != "second line" {
		t.Fatal("multiline entries were not delivered")
	}
}

func TestLoggerは複数Instanceから同じ日次ログへ書き込める(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "paracell.log")
	now := time.Date(2026, time.July, 27, 12, 34, 56, 0, time.Local)
	loggers := []*Logger{NewFile(path), NewFile(path)}
	for _, logger := range loggers {
		logger.now = func() time.Time { return now }
	}

	const writes = 100
	var wg sync.WaitGroup
	for index, logger := range loggers {
		wg.Add(1)
		go func(index int, logger *Logger) {
			defer wg.Done()
			for count := 0; count < writes; count++ {
				if err := logger.Write(LevelInfo, "writer", fmt.Sprintf("%d-%d", index, count)); err != nil {
					t.Errorf("Write failed: %v", err)
					return
				}
			}
		}(index, logger)
	}
	wg.Wait()

	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "paracell-20260727.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "INFO  [writer]"); got != len(loggers)*writes {
		t.Fatalf("saved lines = %d, want %d", got, len(loggers)*writes)
	}
}

func TestFileLoggerは画面購読なしで大量ログを保存できる(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "paracell.log")
	logger := NewFile(path)
	logger.now = func() time.Time { return time.Date(2026, time.July, 27, 0, 0, 0, 0, time.Local) }
	lines := make([]string, 1500)
	for i := range lines {
		lines[i] = "line"
	}

	if err := logger.Write(LevelInfo, "paracell", strings.Join(lines, "\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "paracell-20260727.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "[paracell] line\n"); got != len(lines) {
		t.Fatalf("saved lines = %d, want %d", got, len(lines))
	}
	if logger.Entries() != nil {
		t.Fatal("file logger should not expose a display stream")
	}
}
