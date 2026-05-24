package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shige1114/paradev/internal/domain"
)

type CopyAdapter struct {
	Root string
}

func (a CopyAdapter) CopyFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
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
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
