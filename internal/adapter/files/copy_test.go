package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCopyAdapterはTemplateFilesをSource内の同じ相対Pathへコピーする(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("APP_ENV=dev\n"), 0o644); err != nil {
		t.Fatalf("source fileを書けなかった: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps", "web"), 0o755); err != nil {
		t.Fatalf("source dirを作れなかった: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "web", ".env.local"), []byte("PORT=3000\n"), 0o644); err != nil {
		t.Fatalf("nested source fileを書けなかった: %v", err)
	}
	cellSource := filepath.Join(root, ".paracell", "cells", "123", "source")
	adapter := CopyAdapter{Root: root}

	err := adapter.CopyFiles(context.Background(), domain.Cell{Source: domain.Source{Path: cellSource}}, domain.Template{
		Files: []string{".env", "apps/web/.env.local"},
	})

	if err != nil {
		t.Fatalf("CopyFilesでエラーが返った: %v", err)
	}
	assertFileContent(t, filepath.Join(cellSource, ".env"), "APP_ENV=dev\n")
	assertFileContent(t, filepath.Join(cellSource, "apps", "web", ".env.local"), "PORT=3000\n")
}

func TestCopyAdapterは絶対Pathを拒否する(t *testing.T) {
	adapter := CopyAdapter{Root: t.TempDir()}

	err := adapter.CopyFiles(context.Background(), domain.Cell{}, domain.Template{
		Files: []string{"/tmp/.env"},
	})

	if err == nil {
		t.Fatal("absolute pathなのにエラーが返らなかった")
	}
	if err.Error() != `template file path "/tmp/.env" must be relative` {
		t.Fatalf("error = %q, want %q", err.Error(), `template file path "/tmp/.env" must be relative`)
	}
}

func TestCopyAdapterは親Directoryへ抜けるPathを拒否する(t *testing.T) {
	adapter := CopyAdapter{Root: t.TempDir()}

	err := adapter.CopyFiles(context.Background(), domain.Cell{}, domain.Template{
		Files: []string{"../.env"},
	})

	if err == nil {
		t.Fatal("parent traversal pathなのにエラーが返らなかった")
	}
	if err.Error() != `template file path "../.env" must stay within project root` {
		t.Fatalf("error = %q, want %q", err.Error(), `template file path "../.env" must stay within project root`)
	}
}

func TestCopyAdapterはRetry時に同じ内容の既存Fileを再利用する(t *testing.T) {
	root := t.TempDir()
	cellSource := filepath.Join(root, ".paracell", "cells", "123", "source")
	if err := os.MkdirAll(cellSource, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, ".env"), filepath.Join(cellSource, ".env")} {
		if err := os.WriteFile(path, []byte("same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	adapter := CopyAdapter{Root: root}
	if err := adapter.ResumeFiles(context.Background(), domain.Cell{Source: domain.Source{Path: cellSource}}, domain.Template{Files: []string{".env"}}); err != nil {
		t.Fatalf("same file retry failed: %v", err)
	}
}

func TestCopyAdapterは初回Copyでは既存Fileを従来どおり更新する(t *testing.T) {
	root := t.TempDir()
	cellSource := filepath.Join(root, ".paracell", "cells", "123", "source")
	if err := os.MkdirAll(cellSource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cellSource, ".env")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := CopyAdapter{Root: root}
	if err := adapter.CopyFiles(context.Background(), domain.Cell{Source: domain.Source{Path: cellSource}}, domain.Template{Files: []string{".env"}}); err != nil {
		t.Fatalf("initial copy failed: %v", err)
	}
	assertFileContent(t, target, "template\n")
}

func TestCopyAdapterはRetry時に変更済みFileを上書きしない(t *testing.T) {
	root := t.TempDir()
	cellSource := filepath.Join(root, ".paracell", "cells", "123", "source")
	if err := os.MkdirAll(cellSource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cellSource, ".env")
	if err := os.WriteFile(target, []byte("user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := CopyAdapter{Root: root}
	if err := adapter.ResumeFiles(context.Background(), domain.Cell{Source: domain.Source{Path: cellSource}}, domain.Template{Files: []string{".env"}}); err == nil {
		t.Fatal("changed file was overwritten")
	}
	assertFileContent(t, target, "user change\n")
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s を読めなかった: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}
