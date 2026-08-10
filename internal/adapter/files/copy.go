package files

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

type CopyAdapter struct {
	Root string
}

func (a CopyAdapter) CopyFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
	return a.copyFiles(ctx, cell, template, true)
}

func (a CopyAdapter) ResumeFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
	return a.copyFiles(ctx, cell, template, false)
}

func (a CopyAdapter) copyFiles(ctx context.Context, cell domain.Cell, template domain.Template, overwrite bool) error {
	_ = ctx
	for _, file := range template.Files {
		if filepath.IsAbs(file) {
			return fmt.Errorf("template file path %q must be relative", file)
		}
		clean := filepath.Clean(file)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("template file path %q must stay within project root", file)
		}
		source := filepath.Join(a.Root, clean)
		target := filepath.Join(cell.Source.Path, clean)
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		existing, err := os.ReadFile(target)
		if err == nil {
			if bytes.Equal(existing, data) {
				continue
			}
			if !overwrite {
				return fmt.Errorf("refusing to overwrite existing file %q during retry", target)
			}
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
