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
			Windows: []SessionWindowTemplate{{Name: "editor", Command: "nvim {{.issue}}"}},
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
	if cell.Source.Path != ".paracell/cells/123/source" {
		t.Fatalf("source path = %q, want %q", cell.Source.Path, ".paracell/cells/123/source")
	}
	if cell.Containers.Network != "paracell-myapp-123" {
		t.Fatalf("Docker network名 = %q, want %q", cell.Containers.Network, "paracell-myapp-123")
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "paracell-myapp-123-web" {
		t.Fatalf("webコンテナ名 = %q, want %q", got, "paracell-myapp-123-web")
	}
	if cell.Session.Name != "paracell-myapp-123" {
		t.Fatalf("session名 = %q, want %q", cell.Session.Name, "paracell-myapp-123")
	}
	if len(cell.Session.Windows) != 1 || cell.Session.Windows[0].Command != "nvim {{.issue}}" {
		t.Fatalf("session windows = %#v, want command %q", cell.Session.Windows, "nvim {{.issue}}")
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
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

func TestAggregateRootから子Entityのメソッドを呼び出してコンテナ名を変更する(t *testing.T) {
	cell, err := NewCellFactory().NewCell("cell-1", "123", Template{
		Name: "webapp",
		Containers: ContainerTemplate{
			Services: map[string]ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}, "myapp")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}

	err = cell.RenameContainer("web", "paracell-myapp-123-web-renamed")

	if err != nil {
		t.Fatalf("コンテナ名変更でエラーが返った: %v", err)
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "paracell-myapp-123-web-renamed" {
		t.Fatalf("webコンテナ名 = %q, want %q", got, "paracell-myapp-123-web-renamed")
	}
}

func Test存在しないServiceRoleのコンテナ名変更は失敗する(t *testing.T) {
	cell, err := NewCellFactory().NewCell("cell-1", "123", Template{Name: "webapp"}, "myapp")
	if err != nil {
		t.Fatalf("Cell作成でエラーが返った: %v", err)
	}

	err = cell.RenameContainer("web", "new-name")

	if err == nil {
		t.Fatal("存在しないservice roleなのにエラーが返らなかった")
	}
}

func TestCellはMarkDoneできる(t *testing.T) {
	cell := Cell{}

	if err := cell.MarkDone(); err != nil {
		t.Fatalf("MarkDoneでエラーが返った: %v", err)
	}
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if err := cell.Clean(); err != nil {
		t.Fatalf("Cleanでエラーが返った: %v", err)
	}
}

func TestCellはDone状態を切り替えられる(t *testing.T) {
	cell := Cell{}

	cell.ToggleDone()
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	cell.ToggleDone()
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
}

func TestDoneでないCellはCleanできない(t *testing.T) {
	cell := Cell{}

	if err := cell.Clean(); err == nil {
		t.Fatal("doneでないcellなのにCleanできてしまった")
	}
}
