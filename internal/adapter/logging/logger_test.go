package logging

import (
	"os"
	"path/filepath"
	"strings"
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

	if err := logger.Write(LevelInfo, "git", "stdout: created worktree"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
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

func TestLoggerは5MBを超える前に現在ログをローテーションする(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "paracell.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(MaxFileSize))), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := New(path)
	logger.now = func() time.Time {
		return time.Date(2026, time.July, 27, 12, 34, 56, 123456789, time.Local)
	}

	if err := logger.Write(LevelError, "paracell", "failed"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	rotated := filepath.Join(dir, "logs", "paracell-20260727-123456.123456789.log")
	info, err := os.Stat(rotated)
	if err != nil {
		t.Fatalf("rotated log not found: %v", err)
	}
	if info.Size() != MaxFileSize {
		t.Fatalf("rotated size = %d, want %d", info.Size(), MaxFileSize)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("current log not found: %v", err)
	}
	if !strings.Contains(string(current), "ERROR [paracell] failed") {
		t.Fatalf("current log = %q", current)
	}
}

func TestLoggerは複数行の各行へ時刻レベル処理元を付けて破棄しない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "paracell.log")
	logger := New(path)
	logger.now = func() time.Time { return time.Date(2026, 7, 27, 0, 0, 0, 0, time.Local) }

	if err := logger.Write(LevelError, "paracell", "first line\nsecond line"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
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

func TestLoggerは同じ日時のローテーション済みログを上書きしない(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	path := filepath.Join(logDir, "paracell.log")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 27, 12, 34, 56, 123456789, time.Local)
	existingRotated := filepath.Join(logDir, "paracell-20260727-123456.123456789.log")
	if err := os.WriteFile(existingRotated, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", int(MaxFileSize))), 0o644); err != nil {
		t.Fatal(err)
	}
	logger := New(path)
	logger.now = func() time.Time { return now }

	if err := logger.Write(LevelInfo, "git", "next"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(existingRotated)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("existing rotated log was overwritten: %q", data)
	}
	if _, err := os.Stat(filepath.Join(logDir, "paracell-20260727-123456.123456789-1.log")); err != nil {
		t.Fatalf("collision-safe rotated log not found: %v", err)
	}
}
