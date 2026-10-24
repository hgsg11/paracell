package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

// hooksOnlySettings is the file the issue describes, written with a 2-space indent.
const hooksOnlySettings = `{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "[ -z \"$PARACELL_CELL\" ] || paracell pending"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "[ -z \"$PARACELL_CELL\" ] || paracell ready"
          }
        ]
      }
    ]
  }
}
`

// issueSettings is the JSON shown in issue #152.
const issueSettings = `{
  "hooks": {
    "UserPromptSubmit": [
      { "hooks": [{ "type": "command", "command": "[ -z \"$PARACELL_CELL\" ] || paracell pending" }] }
    ],
    "Stop": [
      { "hooks": [{ "type": "command", "command": "[ -z \"$PARACELL_CELL\" ] || paracell ready" }] }
    ]
  }
}`

const pendingCommand = `[ -z "$PARACELL_CELL" ] || paracell pending`

func TestClaudeSettingsはファイルがないとClaudeディレクトリとHookだけのファイルを作る(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")

	install(t, path)

	got := readFile(t, path)
	if string(got) != hooksOnlySettings {
		t.Fatalf("settings =\n%s\nwant\n%s", got, hooksOnlySettings)
	}
	var gotValue, issueValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(issueSettings), &issueValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, issueValue) {
		t.Fatalf("settings = %#v, want the issue JSON %#v", gotValue, issueValue)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestClaudeSettingsは既存の内容と順序を残してイベントの末尾にHookを足す(t *testing.T) {
	path := writeSettings(t, `{
  "permissions": {"allow": ["Bash(go test:*)"]},
  "env": {"FOO": "bar"},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo pre"}]}
    ],
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [{"type": "command", "command": "echo prompt"}]}
    ]
  }
}`)

	install(t, path)

	want := `{
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  },
  "env": {
    "FOO": "bar"
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo pre"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "echo prompt"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "[ -z \"$PARACELL_CELL\" ] || paracell pending"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "[ -z \"$PARACELL_CELL\" ] || paracell ready"
          }
        ]
      }
    ]
  }
}
`
	if got := readFile(t, path); string(got) != want {
		t.Fatalf("settings =\n%s\nwant\n%s", got, want)
	}
}

func TestClaudeSettingsは2回目に入れてもファイルを書き直さない(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	install(t, path)
	before := snapshotWithOldModTime(t, path)

	install(t, path)

	before.assertUnchanged(t, path)
}

func TestClaudeSettingsは片方だけ入っていると足りない方だけを足す(t *testing.T) {
	path := writeSettings(t, `{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "[ -z \"$PARACELL_CELL\" ] || paracell pending"
          }
        ]
      }
    ]
  }
}
`)

	install(t, path)

	if got := readFile(t, path); string(got) != hooksOnlySettings {
		t.Fatalf("settings =\n%s\nwant\n%s", got, hooksOnlySettings)
	}
}

func TestClaudeSettingsは別の形のHookがあるイベントにもHookを足す(t *testing.T) {
	path := writeSettings(t, `{"hooks": {"UserPromptSubmit": [
  {"matcher": "x"},
  {"hooks": ["text", {"type": "command", "command": 5}, {"type": "prompt", "command": "[ -z \"$PARACELL_CELL\" ] || paracell pending"}]}
]}}`)

	install(t, path)

	got := decodeSettings(t, path)
	entries := got["hooks"].(map[string]any)["UserPromptSubmit"].([]any)
	if len(entries) != 3 {
		t.Fatalf("UserPromptSubmit entries = %d, want 3: %#v", len(entries), entries)
	}
	if !reflect.DeepEqual(entries[2], map[string]any{"hooks": []any{map[string]any{"type": "command", "command": pendingCommand}}}) {
		t.Fatalf("added entry = %#v", entries[2])
	}
}

func TestClaudeSettingsは空のファイルをHookだけのファイルにする(t *testing.T) {
	for name, content := range map[string]string{"0バイト": "", "空白だけ": " \n\t\r\n", "BOMだけ": "\xef\xbb\xbf"} {
		t.Run(name, func(t *testing.T) {
			path := writeSettings(t, content)

			install(t, path)

			if got := readFile(t, path); string(got) != hooksOnlySettings {
				t.Fatalf("settings =\n%s\nwant\n%s", got, hooksOnlySettings)
			}
		})
	}
}

func TestClaudeSettingsは先頭のBOMを読み飛ばして書き戻さない(t *testing.T) {
	path := writeSettings(t, "\xef\xbb\xbf{\"env\": {}}")

	install(t, path)

	got := readFile(t, path)
	if bytes.HasPrefix(got, []byte("\xef\xbb\xbf")) {
		t.Fatalf("settings start with BOM: %q", got)
	}
	if !strings.HasPrefix(string(got), "{\n  \"env\": {},\n  \"hooks\": {") {
		t.Fatalf("settings = %s", got)
	}
}

func TestClaudeSettingsは想定と違うファイルをエラーにして変えない(t *testing.T) {
	cases := map[string]string{
		"壊れたJSON":           `{"hooks": `,
		"末尾のカンマ":            `{"env": {"A": "1",}}`,
		"値の後の余分なデータ":        `{} {}`,
		"トップレベルが配列":         `[]`,
		"トップレベルが文字列":        `"hooks"`,
		"hooksが配列":          `{"hooks": []}`,
		"hooksが文字列":         `{"hooks": "x"}`,
		"イベントの値がオブジェクト":     `{"hooks": {"Stop": {}}}`,
		"イベントの要素がオブジェクトでない": `{"hooks": {"Stop": [1]}}`,
		"要素のhooksが配列でない":    `{"hooks": {"UserPromptSubmit": [{"hooks": {}}]}}`,
		"キーの重複":             `{"env": {}, "env": {}}`,
		"入れ子のキーの重複":         `{"env": {"A": "1", "A": "2"}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeSettings(t, content)
			before := snapshotWithOldModTime(t, path)

			err := ClaudeSettingsAdapter{Path: path}.InstallStatusHooks(context.Background(), domain.AgentStatusHooks())

			if err == nil {
				t.Fatal("想定と違うファイルなのにエラーが返らなかった")
			}
			if !strings.Contains(err.Error(), path) {
				t.Fatalf("error %q does not contain the path %q", err, path)
			}
			before.assertUnchanged(t, path)
			assertOnlySettingsFile(t, path)
		})
	}
}

func TestClaudeSettingsは特殊文字と数値の表記を保つ(t *testing.T) {
	path := writeSettings(t, `{"a": "quote \" backslash \\ lt < gt > amp & 日本語 é", "n": 1.50, "e": 1e3, "b": true, "z": null, "arr": [], "o": {}}`)

	install(t, path)

	got := readFile(t, path)
	wantPrefix := `{
  "a": "quote \" backslash \\ lt < gt > amp & 日本語 é",
  "n": 1.50,
  "e": 1e3,
  "b": true,
  "z": null,
  "arr": [],
  "o": {},
  "hooks": {
`
	if !strings.HasPrefix(string(got), wantPrefix) {
		t.Fatalf("settings =\n%s\nwant prefix\n%s", got, wantPrefix)
	}
	if value := decodeSettings(t, path)["a"]; value != "quote \" backslash \\ lt < gt > amp & 日本語 é" {
		t.Fatalf("a = %q", value)
	}
}

func TestClaudeSettingsは書き込めないとエラーにして元のファイルを変えない(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("rootではディレクトリの権限が効かない")
	}
	path := writeSettings(t, `{"env": {"FOO": "bar"}}`)
	before := snapshotWithOldModTime(t, path)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := ClaudeSettingsAdapter{Path: path}.InstallStatusHooks(context.Background(), domain.AgentStatusHooks())

	if err == nil {
		t.Fatal("書き込めないのにエラーが返らなかった")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q does not contain the path %q", err, path)
	}
	before.assertUnchanged(t, path)
	assertOnlySettingsFile(t, path)
}

func TestClaudeSettingsは既存ファイルのパーミッションを保つ(t *testing.T) {
	path := writeSettings(t, `{}`)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	install(t, path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestClaudeSettingsのコマンドはCellの中だけでParacellを呼ぶ(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	install(t, path)
	hooks := decodeSettings(t, path)["hooks"].(map[string]any)
	commands := map[string]string{
		"UserPromptSubmit": "pending",
		"Stop":             "ready",
	}
	for event, status := range commands {
		cmd := hooks[event].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
		t.Run(event+"/Cellの外", func(t *testing.T) {
			calls := runHookCommand(t, cmd, "")
			if calls != "" {
				t.Fatalf("paracell was called outside a cell: %q", calls)
			}
		})
		t.Run(event+"/Cellの中", func(t *testing.T) {
			calls := runHookCommand(t, cmd, "152")
			if calls != status+"\n" {
				t.Fatalf("paracell calls = %q, want %q", calls, status+"\n")
			}
		})
	}
}

func TestClaudeSettingsは未知のTriggerとStatusをエラーにする(t *testing.T) {
	for name, hook := range map[string]domain.AgentStatusHook{
		"Trigger": {Trigger: domain.AgentTrigger("unknown"), Status: domain.Pending},
		"Status":  {Trigger: domain.TurnStarted, Status: domain.CellStatus("done")},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".claude", "settings.json")

			err := ClaudeSettingsAdapter{Path: path}.InstallStatusHooks(context.Background(), []domain.AgentStatusHook{hook})

			if err == nil {
				t.Fatal("未知の値なのにエラーが返らなかった")
			}
			if _, statErr := os.Stat(filepath.Dir(path)); !os.IsNotExist(statErr) {
				t.Fatalf(".claude was created on error: %v", statErr)
			}
		})
	}
}

// runHookCommand runs cmd as Claude Code would, with a fake paracell on PATH,
// and returns the arguments the fake was called with.
func runHookCommand(t *testing.T, cmd string, cell string) string {
	t.Helper()
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$PARACELL_TEST_CALLS\"\n"
	if err := os.WriteFile(filepath.Join(dir, "paracell"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("sh", "-c", cmd)
	run.Env = append(os.Environ(), "PATH="+dir, "PARACELL_CELL="+cell, "PARACELL_TEST_CALLS="+calls)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("hook command failed: %v: %s", err, out)
	}
	got, err := os.ReadFile(calls)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func install(t *testing.T, path string) {
	t.Helper()
	if err := (ClaudeSettingsAdapter{Path: path}).InstallStatusHooks(context.Background(), domain.AgentStatusHooks()); err != nil {
		t.Fatalf("InstallStatusHooksでエラーが返った: %v", err)
	}
}

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func decodeSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(readFile(t, path), &got); err != nil {
		t.Fatalf("settings are not valid JSON: %v", err)
	}
	return got
}

type fileSnapshot struct {
	content []byte
	modTime time.Time
}

// snapshotWithOldModTime moves the file's modification time into the past, so
// a rewrite within the same clock tick is still detected.
func snapshotWithOldModTime(t *testing.T, path string) fileSnapshot {
	t.Helper()
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return fileSnapshot{content: readFile(t, path), modTime: old}
}

func (s fileSnapshot) assertUnchanged(t *testing.T, path string) {
	t.Helper()
	if got := readFile(t, path); !bytes.Equal(got, s.content) {
		t.Fatalf("settings changed:\n%s\nwant\n%s", got, s.content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(s.modTime) {
		t.Fatalf("modification time = %v, want %v", info.ModTime(), s.modTime)
	}
}

func assertOnlySettingsFile(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("files in %s = %v, want only %s", filepath.Dir(path), names, filepath.Base(path))
	}
}
