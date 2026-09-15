package cell

import (
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestFactoryは完成した配下EntityからCellを生成する(t *testing.T) {
	sourceDriver, _ := domain.NewSourceDriverType("git")
	sessionDriver, _ := domain.NewSessionDriverType("tmux")
	source, _ := domain.NewSource(".", "main", "feat/42")
	target, _ := domain.NewContainer("app", nil, "app", domain.Target)
	dependency, _ := domain.NewContainer("db", nil, "db", domain.Dependency)
	cell, err := NewFactory().NewCell("id", "42", "project", "feat", domain.NewSources(sourceDriver, []domain.Source{source}), domain.NewContainers(domain.Docker, []domain.Container{target, dependency}), domain.NewSession(sessionDriver, nil), domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	if len(cell.Sources.Items) != 1 || cell.Sources.Items[0].Branch != "feat/42" {
		t.Fatalf("sources = %#v", cell.Sources)
	}
	if domain.ContainerResourceName(cell, cell.Containers.Items[0]) == "app" {
		t.Fatal("target container must have a cell-specific name")
	}
	if domain.ContainerResourceName(cell, cell.Containers.Items[1]) != "db" {
		t.Fatal("dependency must keep its source container name")
	}
}
