package domain

import "context"

func CreateSources(ctx context.Context, cell Cell, port SourcePort) (bool, error) {
	created := false
	for _, resource := range cell.sourceResources() {
		branchCreated, err := port.CreateSource(ctx, resource)
		created = created || branchCreated
		if err != nil {
			return created, err
		}
	}
	return created, nil
}

func ResumeSources(ctx context.Context, cell Cell, port SourcePort) error {
	for _, resource := range cell.sourceResources() {
		if err := port.ResumeSource(ctx, resource); err != nil {
			return err
		}
	}
	return nil
}

func CleanSources(ctx context.Context, cell Cell, port SourcePort) error {
	for _, resource := range cell.sourceResources() {
		if err := port.CleanSource(ctx, resource); err != nil {
			return err
		}
	}
	return nil
}
