// Package agent installs Paracell's status hooks into coding agents' settings.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

const (
	newSettingsMode os.FileMode = 0o644
	settingsDirMode os.FileMode = 0o755
	utf8BOM                     = "\xef\xbb\xbf"
)

// ClaudeSettingsAdapter adds the status hooks to a Claude Code settings file
// (the project's .claude/settings.json). It keeps every existing key and its
// order, never adds a hook twice, and leaves the file untouched on any error.
type ClaudeSettingsAdapter struct {
	Path string
}

func NewClaudeSettingsAdapter(path string) ClaudeSettingsAdapter {
	return ClaudeSettingsAdapter{Path: path}
}

func (a ClaudeSettingsAdapter) InstallStatusHooks(ctx context.Context, hooks []domain.AgentStatusHook) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, mode, err := a.read()
	if err != nil {
		return err
	}
	root, err := parseSettings(data)
	if err != nil {
		return fmt.Errorf("claude settings %s: parse: %w", a.Path, err)
	}
	changed := false
	for _, hook := range hooks {
		added, err := addStatusHook(root, hook)
		if err != nil {
			return fmt.Errorf("claude settings %s: %w", a.Path, err)
		}
		changed = changed || added
	}
	if !changed {
		return nil
	}
	return a.writeAtomically(encodeSettings(root), mode)
}

// read returns the current content and permission bits; a missing file reads as empty.
func (a ClaudeSettingsAdapter) read() ([]byte, os.FileMode, error) {
	info, err := os.Stat(a.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, newSettingsMode, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("claude settings %s: stat: %w", a.Path, err)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return nil, 0, fmt.Errorf("claude settings %s: read: %w", a.Path, err)
	}
	return data, info.Mode().Perm(), nil
}

// writeAtomically writes a temporary file next to the settings and renames it
// over them, so a failure never leaves a partly written settings file.
func (a ClaudeSettingsAdapter) writeAtomically(data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, settingsDirMode); err != nil {
		return fmt.Errorf("claude settings %s: create directory: %w", a.Path, err)
	}
	tmp, err := os.CreateTemp(dir, ".settings.json.*.tmp")
	if err != nil {
		return fmt.Errorf("claude settings %s: create temporary file: %w", a.Path, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			err = errors.Join(err, removeIfExists(tmp.Name()))
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("claude settings %s: write temporary file: %w", a.Path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("claude settings %s: sync temporary file: %w", a.Path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("claude settings %s: close temporary file: %w", a.Path, err)
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("claude settings %s: chmod temporary file: %w", a.Path, err)
	}
	if err := os.Rename(tmp.Name(), a.Path); err != nil {
		return fmt.Errorf("claude settings %s: replace: %w", a.Path, err)
	}
	return nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove temporary file %s: %w", path, err)
	}
	return nil
}

// addStatusHook appends the hook to its event unless a command hook with the
// same command is already there. It reports whether the settings changed.
func addStatusHook(root *jsonObject, hook domain.AgentStatusHook) (bool, error) {
	event, err := claudeEvent(hook.Trigger)
	if err != nil {
		return false, err
	}
	cmd, err := command(hook.Status)
	if err != nil {
		return false, err
	}
	hooksValue := root.getOrAdd("hooks", &jsonObject{})
	hooks, ok := hooksValue.(*jsonObject)
	if !ok {
		return false, errors.New(`"hooks" is not an object`)
	}
	entries, ok := hooks.getOrAdd(event, &jsonArray{}).(*jsonArray)
	if !ok {
		return false, fmt.Errorf(`"hooks.%s" is not an array`, event)
	}
	found, err := hasCommandHook(entries, event, cmd)
	if err != nil || found {
		return false, err
	}
	entries.items = append(entries.items, newCommandEntry(cmd))
	return true, nil
}

// hasCommandHook checks every entry of an event, so a malformed entry is an
// error even when the command is already present in an earlier entry.
func hasCommandHook(entries *jsonArray, event string, cmd string) (bool, error) {
	found := false
	for i, item := range entries.items {
		entry, ok := item.(*jsonObject)
		if !ok {
			return false, fmt.Errorf(`"hooks.%s[%d]" is not an object`, event, i)
		}
		value, ok := entry.get("hooks")
		if !ok {
			continue
		}
		handlers, ok := value.(*jsonArray)
		if !ok {
			return false, fmt.Errorf(`"hooks.%s[%d].hooks" is not an array`, event, i)
		}
		found = found || containsCommand(handlers, cmd)
	}
	return found, nil
}

// containsCommand skips handlers of other shapes instead of failing, so hooks
// the user wrote in another form are left as they are.
func containsCommand(handlers *jsonArray, cmd string) bool {
	for _, item := range handlers.items {
		handler, ok := item.(*jsonObject)
		if !ok {
			continue
		}
		kind, _ := handler.get("type")
		value, _ := handler.get("command")
		if kind == "command" && value == cmd {
			return true
		}
	}
	return false
}

func newCommandEntry(cmd string) *jsonObject {
	handler := &jsonObject{members: []jsonMember{{key: "type", value: "command"}, {key: "command", value: cmd}}}
	return &jsonObject{members: []jsonMember{{key: "hooks", value: &jsonArray{items: []jsonValue{handler}}}}}
}

// claudeEvent maps an agent-independent trigger to a Claude Code hook event.
func claudeEvent(trigger domain.AgentTrigger) (string, error) {
	switch trigger {
	case domain.TurnStarted:
		return "UserPromptSubmit", nil
	case domain.TurnFinished:
		return "Stop", nil
	default:
		return "", fmt.Errorf("unsupported agent trigger %q", trigger)
	}
}

// command returns the shell command for a status. Outside a Cell
// (PARACELL_CELL is empty) it does nothing and exits 0.
func command(status domain.CellStatus) (string, error) {
	switch status {
	case domain.Pending, domain.Ready:
		return `[ -z "$PARACELL_CELL" ] || paracell ` + string(status), nil
	default:
		return "", fmt.Errorf("unsupported status %q", status)
	}
}

// jsonValue is one of *jsonObject, *jsonArray, string, json.Number, bool or nil.
type jsonValue any

type jsonMember struct {
	key   string
	value jsonValue
}

// jsonObject keeps its members in file order.
type jsonObject struct {
	members []jsonMember
}

type jsonArray struct {
	items []jsonValue
}

func (o *jsonObject) get(key string) (jsonValue, bool) {
	for _, member := range o.members {
		if member.key == key {
			return member.value, true
		}
	}
	return nil, false
}

// getOrAdd returns the value of key, appending it with value when it is missing.
func (o *jsonObject) getOrAdd(key string, value jsonValue) jsonValue {
	if existing, ok := o.get(key); ok {
		return existing
	}
	o.members = append(o.members, jsonMember{key: key, value: value})
	return value
}

// parseSettings reads the settings as an ordered tree. An empty or
// whitespace-only file is an empty object; the top level must be an object.
func parseSettings(data []byte) (*jsonObject, error) {
	data = bytes.TrimPrefix(data, []byte(utf8BOM))
	if strings.Trim(string(data), " \t\r\n") == "" {
		return &jsonObject{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the top-level value")
	}
	root, ok := value.(*jsonObject)
	if !ok {
		return nil, errors.New("top-level value is not an object")
	}
	return root, nil
}

func parseValue(dec *json.Decoder) (jsonValue, error) {
	tok, err := dec.Token()
	if errors.Is(err, io.EOF) {
		return nil, io.ErrUnexpectedEOF
	}
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return parseObject(dec)
		}
		if v == '[' {
			return parseArray(dec)
		}
		return nil, fmt.Errorf("unexpected %q", rune(v))
	default:
		return v, nil
	}
}

func parseObject(dec *json.Decoder) (*jsonObject, error) {
	obj := &jsonObject{}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("object key %v is not a string", tok)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		value, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		obj.members = append(obj.members, jsonMember{key: key, value: value})
	}
	return obj, expectDelim(dec, '}')
}

func parseArray(dec *json.Decoder) (*jsonArray, error) {
	arr := &jsonArray{}
	for dec.More() {
		value, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		arr.items = append(arr.items, value)
	}
	return arr, expectDelim(dec, ']')
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	if err != nil {
		return err
	}
	if tok != want {
		return fmt.Errorf("expected %q, got %v", rune(want), tok)
	}
	return nil
}

// encodeSettings writes the tree with a 2-space indent and a trailing newline.
func encodeSettings(root *jsonObject) []byte {
	var buf bytes.Buffer
	writeValue(&buf, root, "")
	buf.WriteByte('\n')
	return buf.Bytes()
}

func writeValue(buf *bytes.Buffer, value jsonValue, indent string) {
	switch v := value.(type) {
	case *jsonObject:
		writeObject(buf, v, indent)
	case *jsonArray:
		writeArray(buf, v, indent)
	case string:
		buf.WriteString(encodeString(v))
	case json.Number:
		buf.WriteString(string(v))
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	default:
		buf.WriteString("null")
	}
}

func writeObject(buf *bytes.Buffer, obj *jsonObject, indent string) {
	if len(obj.members) == 0 {
		buf.WriteString("{}")
		return
	}
	inner := indent + "  "
	buf.WriteString("{\n")
	for i, member := range obj.members {
		buf.WriteString(inner + encodeString(member.key) + ": ")
		writeValue(buf, member.value, inner)
		if i < len(obj.members)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString(indent + "}")
}

func writeArray(buf *bytes.Buffer, arr *jsonArray, indent string) {
	if len(arr.items) == 0 {
		buf.WriteString("[]")
		return
	}
	inner := indent + "  "
	buf.WriteString("[\n")
	for i, item := range arr.items {
		buf.WriteString(inner)
		writeValue(buf, item, inner)
		if i < len(arr.items)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString(indent + "]")
}

// encodeString quotes s as JSON without HTML escaping, so "<" and "&" stay as written.
func encodeString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// Encoding a Go string cannot fail.
	_ = enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}
