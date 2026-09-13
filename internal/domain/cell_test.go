package domain

import (
	"strings"
	"testing"
)

func TestCellNoteは空白を正規化してUnicode文字数で検証する(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "空白正規化", input: "  API\t実装\n  中  ", want: "API 実装 中"},
		{name: "1文字", input: "案", want: "案"},
		{name: "20文字", input: strings.Repeat("案", 20), want: strings.Repeat("案", 20)},
		{name: "空文字", input: " \t\n ", wantErr: true},
		{name: "21文字", input: strings.Repeat("案", 21), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCellNote(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeCellNote() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("NormalizeCellNote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCellNoteは設定時だけ表示を置き換える(t *testing.T) {
	withoutNote := Cell{Name: "123"}
	withNote := Cell{Name: "123", Note: "API実装中"}
	if got := withoutNote.DisplayLabel(); got != "123" {
		t.Fatalf("DisplayLabel() = %q, want 123", got)
	}
	if got := withNote.DisplayLabel(); got != "API実装中" {
		t.Fatalf("DisplayLabel() = %q, want API実装中", got)
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
	cell := Cell{
		Containers: Containers{
			Services: map[string]CellContainer{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}

	err := cell.RenameContainer("web", "paracell-myapp-123-web-renamed")

	if err != nil {
		t.Fatalf("コンテナ名変更でエラーが返った: %v", err)
	}
	if got := cell.Containers.Services["web"].ContainerName; got != "paracell-myapp-123-web-renamed" {
		t.Fatalf("webコンテナ名 = %q, want %q", got, "paracell-myapp-123-web-renamed")
	}
}

func Test存在しないServiceRoleのコンテナ名変更は失敗する(t *testing.T) {
	cell := Cell{}

	err := cell.RenameContainer("web", "new-name")

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

func TestCellはStatusを更新できる(t *testing.T) {
	cell := Cell{}

	if err := cell.SetStatus(Ready); err != nil {
		t.Fatalf("SetStatusでエラーが返った: %v", err)
	}
	if got := cell.Status(); got != Ready {
		t.Fatalf("Status = %q, want %q", got, Ready)
	}
}

func TestCellは未対応Statusを拒否する(t *testing.T) {
	cell := Cell{}

	err := cell.SetStatus(CellStatus("running"))

	if err == nil {
		t.Fatal("未対応statusなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported status "running"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported status "running"`)
	}
}
