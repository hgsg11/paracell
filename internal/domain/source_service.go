package domain

import "context"

func CreateSources(ctx context.Context, cell *Cell, driver SourceDriverType, templates []SourceTemplate, issue string, port SourcePort) error {
	sources, err := buildSources(driver, templates, issue)
	if err != nil {
		return err
	}
	cell.PlanSources(sources)
	for _, resource := range cell.sourceResources() {
		if err := port.CreateSource(ctx, resource); err != nil {
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
