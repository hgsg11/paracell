package domain

import (
	"reflect"
	"testing"
)

func TestAgentStatusHooksは作業開始でPendingと応答後にReadyをこの順で返す(t *testing.T) {
	got := AgentStatusHooks()

	want := []AgentStatusHook{
		{Trigger: TurnStarted, Status: Pending},
		{Trigger: TurnFinished, Status: Ready},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hooks = %#v, want %#v", got, want)
	}
}

func TestAgentStatusHooksの戻り値を書き換えても次の呼び出しに影響しない(t *testing.T) {
	first := AgentStatusHooks()
	first[0] = AgentStatusHook{Trigger: TurnFinished, Status: Ready}

	second := AgentStatusHooks()

	if second[0].Trigger != TurnStarted || second[0].Status != Pending {
		t.Fatalf("hooks[0] = %#v, want turn-started/pending", second[0])
	}
}

func TestAgentTriggerは作業の開始と終了を受け付ける(t *testing.T) {
	for _, value := range []string{"turn-started", "turn-finished"} {
		got, err := NewAgentTrigger(value)
		if err != nil {
			t.Fatalf("NewAgentTrigger(%q)でエラーが返った: %v", value, err)
		}
		if string(got) != value {
			t.Fatalf("trigger = %q, want %q", got, value)
		}
	}
}

func TestAgentTriggerは空文字と未知の値をエラーにする(t *testing.T) {
	for _, value := range []string{"", "UserPromptSubmit", "turn-start"} {
		if _, err := NewAgentTrigger(value); err == nil {
			t.Fatalf("NewAgentTrigger(%q)でエラーが返らなかった", value)
		}
	}
}
