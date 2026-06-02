package system

import (
	"context"
	"strings"
	"testing"
)

func TestCaptureRunnerはstderrをエラーに含める(t *testing.T) {
	runner := CaptureRunner{Dir: t.TempDir()}

	err := runner.Run(context.Background(), "sh", "-c", "echo boom >&2; exit 1")

	if err == nil {
		t.Fatal("エラーが返らなかった")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want stderrを含む", err.Error())
	}
}
