package cell

import (
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestテンプレートからCellを作成できる(t *testing.T) {
	factory := Factory{}
	template := domain.Template{
		Name: "webapp",
		Repository: domain.RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "main",
			BranchMode:   "reuse",
		},
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
		Session: domain.SessionTemplate{
			Windows: []domain.SessionWindowTemplate{{Name: "editor", Command: "nvim {{.issue}}"}},
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
	if cell.Base != "main" {
		t.Fatalf("Base = %q, want %q", cell.Base, "main")
	}
	if cell.Branch != "feat/123" {
		t.Fatalf("ブランチ名 = %q, want %q", cell.Branch, "feat/123")
	}
	if cell.BranchMode != "reuse" {
		t.Fatalf("branch mode = %q, want %q", cell.BranchMode, "reuse")
	}
	if cell.Source.Path != ".paracell/cells/123/source" {
		t.Fatalf("source path = %q, want %q", cell.Source.Path, ".paracell/cells/123/source")
	}
	if cell.Containers.Network != "paracell-myapp-123" {
		t.Fatalf("Docker network名 = %q, want %q", cell.Containers.Network, "paracell-myapp-123")
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "paracell-myapp-123-web" {
		t.Fatalf("webコンテナ名 = %q, want %q", got, "paracell-myapp-123-web")
	}
	if cell.Session.Name != "myapp-123" {
		t.Fatalf("session名 = %q, want %q", cell.Session.Name, "myapp-123")
	}
	if len(cell.Session.Windows) != 1 || cell.Session.Windows[0].Command != "nvim {{.issue}}" {
		t.Fatalf("session windows = %#v, want command %q", cell.Session.Windows, "nvim {{.issue}}")
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
	if got := cell.Status(); got != domain.Ready {
		t.Fatalf("Status = %q, want %q", got, domain.Ready)
	}
}

func TestテンプレートのBaseをCellへ保持する(t *testing.T) {
	factory := Factory{}
	template := domain.Template{
		Name: "webapp",
		Repository: domain.RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "current",
		},
	}

	cell, err := factory.NewCell("cell-1", "123", template, "myapp")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}
	if cell.Base != "current" {
		t.Fatalf("cell base = %q, want %q", cell.Base, "current")
	}
}

func TestIssueの不正文字をリソース名では安全なハイフンへ変換する(t *testing.T) {
	factory := Factory{}
	template := domain.Template{
		Name: "webapp",
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{"web": {}},
		},
	}

	cell, err := factory.NewCell("cell-1", `feature\volume / copy:?*[test]`, template, "my app")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}
	if cell.Issue != `feature\volume / copy:?*[test]` {
		t.Fatalf("Issue = %q, want original issue", cell.Issue)
	}
	if cell.Name != "feature-volume-copy-test" {
		t.Fatalf("Cell名 = %q, want %q", cell.Name, "feature-volume-copy-test")
	}
	if cell.Source.Path != ".paracell/cells/feature-volume-copy-test/source" {
		t.Fatalf("source path = %q", cell.Source.Path)
	}
	if cell.Containers.Network != "paracell-my-app-feature-volume-copy-test" {
		t.Fatalf("Docker network名 = %q", cell.Containers.Network)
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "paracell-my-app-feature-volume-copy-test-web" {
		t.Fatalf("webコンテナ名 = %q", got)
	}
	if cell.Session.Name != "my-app-feature-volume-copy-test" {
		t.Fatalf("session名 = %q", cell.Session.Name)
	}
	if cell.Branch != `feature\volume / copy:?*[test]` {
		t.Fatalf("branch = %q, want original issue", cell.Branch)
	}
}

func TestManagedDockerResource名はProjectCellServiceRoleで分離する(t *testing.T) {
	factory := Factory{}
	template := domain.Template{
		Name: "webapp",
		Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
			"frontend": {},
			"backend":  {},
		}},
	}

	first, err := factory.NewCell("cell-67", "67", template, "paracell")
	if err != nil {
		t.Fatalf("first cell creation failed: %v", err)
	}
	second, err := factory.NewCell("cell-68", "68", template, "paracell")
	if err != nil {
		t.Fatalf("second cell creation failed: %v", err)
	}

	if first.Containers.Network != "paracell-paracell-67" {
		t.Fatalf("first network = %q", first.Containers.Network)
	}
	if second.Containers.Network != "paracell-paracell-68" {
		t.Fatalf("second network = %q", second.Containers.Network)
	}
	for role, want := range map[string]string{
		"frontend": "paracell-paracell-67-frontend",
		"backend":  "paracell-paracell-67-backend",
	} {
		if got := first.Containers.Services[role].ContainerName; got != want {
			t.Fatalf("%s container = %q, want %q", role, got, want)
		}
	}
}
