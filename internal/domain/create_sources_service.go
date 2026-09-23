package domain

import "context"

func CreateSourcesService(ctx context.Context, cell Cell, templates []SourceTemplate, create func(context.Context, SourceTemplate, string, string) error) error {
	for i, template := range templates {
		source := cell.Sources.Items[i]
		if err := create(ctx, template, cell.SourceWorktreePath(source), source.Branch); err != nil {
			return err
		}
	}
	return nil
}
