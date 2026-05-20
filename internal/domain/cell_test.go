package domain

import "testing"

func TestテンプレートからCellを作成できる(t *testing.T) {
	factory := NewCellFactory()
	template := Template{
		Name: "webapp",
		Repository: RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "main",
		},
		Containers: ContainerTemplate{
			Services: map[string]ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
		Session: SessionTemplate{
			Windows: []SessionWindowTemplate{{Name: "editor", Command: "nvim ."}},
		},
	}

	cell, err := factory.NewCell("cell-1", "123", template, "myapp")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}
	if cell.ID != "cell-1" {
		t.Fatalf("Cell ID = %q, want %q", cell.ID, "cell-1")
	}
	if cell.Name != "123" {
		t.Fatalf("Cell名 = %q, want %q", cell.Name, "123")
	}
	if cell.Branch != "feat/123" {
		t.Fatalf("ブランチ名 = %q, want %q", cell.Branch, "feat/123")
	}
	if cell.Source.Path != ".pdev/cells/123/source" {
		t.Fatalf("source path = %q, want %q", cell.Source.Path, ".pdev/cells/123/source")
	}
	if cell.Containers.Network != "pdev-myapp-123" {
		t.Fatalf("Docker network名 = %q, want %q", cell.Containers.Network, "pdev-myapp-123")
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "pdev-myapp-123-web" {
		t.Fatalf("webコンテナ名 = %q, want %q", got, "pdev-myapp-123-web")
	}
	if cell.Session.Name != "pdev-myapp-123" {
		t.Fatalf("session名 = %q, want %q", cell.Session.Name, "pdev-myapp-123")
	}
}

func Test同じIssueのCellは重複として扱う(t *testing.T) {
	checker := CellUniquenessChecker{}
	existing := []Cell{{Issue: "123", Name: "123"}}

	err := checker.EnsureUnique(existing, "123", "123")

	if err == nil {
		t.Fatal("重複しているのにエラーが返らなかった")
	}
}
