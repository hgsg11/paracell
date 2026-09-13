package cell

import (
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestFactoryはTemplateからCellを生成する(t *testing.T) {
	source, _ := domain.NewSourceTemplate(".", "main", "feat/")
	target, _ := domain.NewContainerTemplate("app", domain.Target, nil, nil)
	dependency, _ := domain.NewContainerTemplate("db", domain.Dependency, nil, nil)
	cell, err := (Factory{}).NewCell("id", "42", "feat", []domain.SourceTemplate{source}, []domain.ContainerTemplate{target, dependency}, domain.NewSessionTemplate(nil), "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(cell.Sources) != 1 || cell.Sources[0].Branch != "feat/42" {
		t.Fatalf("sources = %#v", cell.Sources)
	}
	if cell.Containers.Services["app"].ContainerName == "app" {
		t.Fatal("target container must have a cell-specific name")
	}
	if cell.Containers.Services["db"].ContainerName != "db" {
		t.Fatal("dependency must keep its source container name")
	}
}
