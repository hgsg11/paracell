package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

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
