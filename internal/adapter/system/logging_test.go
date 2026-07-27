package system

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/logging"
)

func TestLoggingRunnerはstdoutとstderrをリアルタイム配信する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	runner := LoggingRunner{Dir: t.TempDir(), Logger: logger}
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(context.Background(), "sh", "-c", "printf out; printf err >&2; sleep 0.5")
	}()

	deadline := time.After(300 * time.Millisecond)
	contents := make([]string, 0, 3)
	for len(contents) < 3 {
		select {
		case entry := <-logger.Entries():
			contents = append(contents, entry.Content)
		case <-deadline:
			t.Fatalf("output was not delivered before command completion: %v", contents)
		}
	}
	if !contains(contents, "stdout: out") || !contains(contents, "stderr: err") {
		t.Fatalf("entries = %v", contents)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestLoggingRunnerは成功時も開始と完了を記録する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	runner := LoggingRunner{Dir: t.TempDir(), Logger: logger}

	if err := runner.Run(context.Background(), "sh", "-c", "true"); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	first := <-logger.Entries()
	second := <-logger.Entries()
	if first.Content != "started" || second.Content != "completed" {
		t.Fatalf("entries = %q, %q", first.Content, second.Content)
	}
}

func TestLoggingRunnerはOutputを返しつつ全行を記録する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	runner := LoggingRunner{Dir: t.TempDir(), Logger: logger}

	output, err := runner.Output(context.Background(), "sh", "-c", "printf 'one\\ntwo\\n'")
	if err != nil {
		t.Fatalf("Output failed: %v", err)
	}
	if output != "one\ntwo" {
		t.Fatalf("output = %q", output)
	}
	data := drainEntries(logger.Entries(), 4)
	if !strings.Contains(data, "stdout: one") || !strings.Contains(data, "stdout: two") {
		t.Fatalf("entries = %q", data)
	}
}

func TestLoggingRunnerは失敗時もstdoutとstderrとエラーを保持する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	runner := LoggingRunner{Dir: t.TempDir(), Logger: logger}

	err := runner.Run(context.Background(), "sh", "-c", "echo partial; echo boom >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want stderr", err)
	}
	data := drainEntries(logger.Entries(), 4)
	for _, want := range []string{"stdout: partial", "stderr: boom", "failed: exit status 7: boom"} {
		if !strings.Contains(data, want) {
			t.Fatalf("entries = %q, want %q", data, want)
		}
	}
}

func TestLoggingRunnerは対話処理の標準入出力も接続したまま記録する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	var terminal bytes.Buffer
	runner := LoggingRunner{
		Dir:    t.TempDir(),
		Logger: logger,
		Stdin:  strings.NewReader("input\n"),
		Stdout: &terminal,
	}

	if err := runner.Run(context.Background(), "sh", "-c", "read value; printf received-%s \"$value\""); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if terminal.String() != "received-input" {
		t.Fatalf("terminal output = %q", terminal.String())
	}
	data := drainEntries(logger.Entries(), 3)
	if !strings.Contains(data, "stdout: received-input") {
		t.Fatalf("entries = %q", data)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func drainEntries(entries <-chan logging.Entry, count int) string {
	lines := make([]string, 0, count)
	for range count {
		lines = append(lines, (<-entries).String())
	}
	return strings.Join(lines, "\n")
}
