package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestCellNoteの表示規則を共通化する(t *testing.T) {
	withoutNote := Cell{Name: "123"}
	withNote := Cell{Name: "123", Note: "API実装中"}
	if got := withoutNote.DisplayLabel(); got != "123" {
		t.Fatalf("DisplayLabel() = %q, want 123", got)
	}
	if got := withNote.DisplayLabel(); got != "API実装中" {
		t.Fatalf("DisplayLabel() = %q, want API実装中", got)
	}
	if got := withNote.TUIDisplayLabel(); got != "123 | API実装中" {
		t.Fatalf("TUIDisplayLabel() = %q, want %q", got, "123 | API実装中")
	}
}

func TestCellNoteはJSONをRoundTripし旧JSONでは空になる(t *testing.T) {
	original := Cell{ID: "cell-1", Issue: "123", Name: "123", Note: "API実装中"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Cell
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Note != original.Note {
		t.Fatalf("Note = %q, want %q", decoded.Note, original.Note)
	}
	if err := json.Unmarshal([]byte(`{"id":"legacy","issue":"1","name":"legacy"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Note != "" || decoded.DisplayLabel() != "legacy" {
		t.Fatalf("legacy cell = %#v, want empty note and name fallback", decoded)
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

func TestCellは作成LifecycleとCheckpointをJSONでRoundTripできる(t *testing.T) {
	cell := Cell{ID: "cell-1", Issue: "73", Name: "73", Template: "fix"}
	cell.BeginCreation("read issue")
	cell.CompleteCreationStage(CreationStageSource)
	wantErr := errors.New("copy failed")
	cell.FailCreation(CreationStageFiles, wantErr)

	data, err := json.Marshal(cell)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Cell
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.CreationStatus() != CreationFailed || !reflect.DeepEqual(decoded.Creation, cell.Creation) {
		t.Fatalf("creation = %#v, want %#v", decoded.Creation, cell.Creation)
	}
}

func TestCellはCreation情報のない旧JSONをReadyとして扱う(t *testing.T) {
	var cell Cell
	if err := json.Unmarshal([]byte(`{"id":"legacy","name":"legacy","status":"pending"}`), &cell); err != nil {
		t.Fatal(err)
	}
	if cell.CreationStatus() != CreationReady || cell.Status() != Pending {
		t.Fatalf("creation=%q status=%q", cell.CreationStatus(), cell.Status())
	}
}

func TestCellはRetryLeaseをUTCのJSONでRoundTripできる(t *testing.T) {
	started := time.Date(2026, 8, 10, 12, 34, 56, 0, time.FixedZone("JST", 9*60*60))
	cell := Cell{ID: "cell-1", Issue: "76", Name: "76"}
	cell.BeginCreation("retry safely")
	cell.FailCreation(CreationStageContainers, errors.New("docker failed"))
	cell.BeginRetry("attempt-1", started)

	data, err := json.Marshal(cell)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "+09:00") || !strings.Contains(string(data), "2026-08-10T03:34:56Z") {
		t.Fatalf("lease timestamp is not UTC: %s", data)
	}
	var decoded Cell
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Creation.AttemptID != "attempt-1" || decoded.Creation.LeaseHeartbeatAt == nil || !decoded.RetryLeaseValid(started.Add(2*time.Minute), 2*time.Minute) {
		t.Fatalf("decoded lease = %#v", decoded.Creation)
	}
}

func TestCellはLease情報のない旧RetryingJSONをStaleとして扱う(t *testing.T) {
	var cell Cell
	if err := json.Unmarshal([]byte(`{"id":"legacy","name":"legacy","creation":{"status":"retrying","failedStage":"files"}}`), &cell); err != nil {
		t.Fatal(err)
	}
	if cell.CreationStatus() != CreationRetrying || cell.RetryLeaseValid(time.Now(), 2*time.Minute) {
		t.Fatalf("legacy retry state = %#v", cell.Creation)
	}
}
