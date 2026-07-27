package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/hgsg11/paracell/internal/adapter/logging"
)

type LoggingRunner struct {
	Dir    string
	Logger *logging.Logger
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (r LoggingRunner) Run(ctx context.Context, name string, args ...string) error {
	_, err := r.execute(ctx, false, name, args...)
	return err
}

func (r LoggingRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	return r.execute(ctx, true, name, args...)
}

func (r LoggingRunner) execute(ctx context.Context, captureStdout bool, name string, args ...string) (string, error) {
	if r.Logger == nil {
		return "", errors.New("logging runner requires a logger")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	cmd.Stdin = r.Stdin

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutWriter := newStreamWriter(func(line string) error {
		return r.Logger.Write(logging.LevelInfo, name, "stdout: "+line)
	})
	stderrWriter := newStreamWriter(func(line string) error {
		return r.Logger.Write(logging.LevelError, name, "stderr: "+line)
	})
	if captureStdout {
		cmd.Stdout = io.MultiWriter(&stdout, stdoutWriter)
	} else if r.Stdout != nil {
		cmd.Stdout = io.MultiWriter(r.Stdout, stdoutWriter)
	} else {
		cmd.Stdout = stdoutWriter
	}
	if r.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderr, r.Stderr, stderrWriter)
	} else {
		cmd.Stderr = io.MultiWriter(&stderr, stderrWriter)
	}

	if err := r.Logger.Write(logging.LevelInfo, name, "started"); err != nil {
		return "", err
	}
	commandErr := cmd.Run()
	logErr := errors.Join(stdoutWriter.Err(), stderrWriter.Err())
	if commandErr != nil {
		message := "failed: " + commandErr.Error()
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			message += ": " + detail
		}
		if err := r.Logger.Write(logging.LevelError, name, message); err != nil {
			logErr = errors.Join(logErr, err)
		}
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			commandErr = fmt.Errorf("%w: %s", commandErr, detail)
		}
		return strings.TrimSpace(stdout.String()), errors.Join(commandErr, logErr)
	}
	if err := r.Logger.Write(logging.LevelInfo, name, "completed"); err != nil {
		logErr = errors.Join(logErr, err)
	}
	return strings.TrimSpace(stdout.String()), logErr
}

type streamWriter struct {
	mu    sync.Mutex
	write func(string) error
	err   error
}

func newStreamWriter(write func(string) error) *streamWriter {
	return &streamWriter{write: write}
}

func (w *streamWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	content := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		w.err = errors.Join(w.err, w.write(strings.TrimSuffix(line, "\r")))
	}
	return len(data), nil
}

func (w *streamWriter) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}
