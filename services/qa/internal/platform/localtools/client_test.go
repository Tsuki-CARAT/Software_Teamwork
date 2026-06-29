package localtools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, enableCommand bool) *Client {
	t.Helper()
	client, err := New(Config{
		WorkDir:           t.TempDir(),
		MaxFileBytes:      1024,
		MaxOutputBytes:    1024,
		EnableCommandTool: enableCommand,
		CommandTimeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestFileToolsUseFunctionCallingAndStayInWorkspace(t *testing.T) {
	client := newTestClient(t, false)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Fatalf("tools = %d, want 3", len(tools))
	}
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function.Name == "" || tool.Function.Parameters == nil {
			t.Fatalf("invalid function tool: %+v", tool)
		}
	}

	write, err := client.CallTool(context.Background(), ToolWriteFile, json.RawMessage(`{"path":"notes/a.txt","content":"first\nsecond\nthird"}`))
	if err != nil || write.IsError {
		t.Fatalf("write failed: result=%+v err=%v", write, err)
	}
	read, err := client.CallTool(context.Background(), ToolReadFile, json.RawMessage(`{"path":"notes/a.txt","limit":2}`))
	if err != nil || read.IsError || !strings.Contains(read.Content, "1 more lines") {
		t.Fatalf("read failed: result=%+v err=%v", read, err)
	}
	edit, err := client.CallTool(context.Background(), ToolEditFile, json.RawMessage(`{"path":"notes/a.txt","old_text":"second","new_text":"changed"}`))
	if err != nil || edit.IsError {
		t.Fatalf("edit failed: result=%+v err=%v", edit, err)
	}
	data, err := os.ReadFile(filepath.Join(client.root, "notes", "a.txt"))
	if err != nil || string(data) != "first\nchanged\nthird" {
		t.Fatalf("unexpected edited file: %q, err=%v", data, err)
	}

	escape, err := client.CallTool(context.Background(), ToolWriteFile, json.RawMessage(`{"path":"../escape.txt","content":"no"}`))
	if err != nil || !escape.IsError || !strings.Contains(escape.Content, "invalid_path") {
		t.Fatalf("workspace escape was not blocked: result=%+v err=%v", escape, err)
	}
}

func TestFileToolsRejectSymlinkEscape(t *testing.T) {
	client := newTestClient(t, false)
	outside := t.TempDir()
	link := filepath.Join(client.root, "outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	result, err := client.CallTool(context.Background(), ToolWriteFile, json.RawMessage(`{"path":"outside/escape.txt","content":"no"}`))
	if err != nil || !result.IsError || !strings.Contains(result.Content, "invalid_path") {
		t.Fatalf("symlink escape was not blocked: result=%+v err=%v", result, err)
	}
}

func TestReadFileDistinguishesUTF8TruncationFromInvalidContent(t *testing.T) {
	root := t.TempDir()
	client, err := New(Config{
		WorkDir:        root,
		MaxFileBytes:   5,
		MaxOutputBytes: 1024,
		CommandTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "valid.txt"), []byte("你好"), 0o644); err != nil {
		t.Fatal(err)
	}
	valid, err := client.CallTool(context.Background(), ToolReadFile, json.RawMessage(`{"path":"valid.txt"}`))
	if err != nil || valid.IsError || !strings.Contains(valid.Content, "你") || !strings.Contains(valid.Content, "file truncated") {
		t.Fatalf("partial UTF-8 rune was not handled: result=%+v err=%v", valid, err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid.txt"), []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa}, 0o644); err != nil {
		t.Fatal(err)
	}
	invalid, err := client.CallTool(context.Background(), ToolReadFile, json.RawMessage(`{"path":"invalid.txt"}`))
	if err != nil || !invalid.IsError || !strings.Contains(invalid.Content, "unsupported_encoding") {
		t.Fatalf("invalid UTF-8 was not rejected: result=%+v err=%v", invalid, err)
	}
}

func TestEditFileRejectsOversizedInput(t *testing.T) {
	client := newTestClient(t, false)
	if err := os.WriteFile(filepath.Join(client.root, "large.txt"), []byte(strings.Repeat("x", 2048)), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := client.CallTool(context.Background(), ToolEditFile, json.RawMessage(`{"path":"large.txt","old_text":"x","new_text":"y"}`))
	if err != nil || !result.IsError || !strings.Contains(result.Content, "file_too_large") {
		t.Fatalf("oversized edit was not rejected: result=%+v err=%v", result, err)
	}
}

func TestCommandToolIsOptInAndBounded(t *testing.T) {
	disabled := newTestClient(t, false)
	tools, err := disabled.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Function.Name == ToolBash {
			t.Fatal("bash tool must not be exposed by default")
		}
	}

	enabled := newTestClient(t, true)
	result, err := enabled.CallTool(context.Background(), ToolBash, json.RawMessage(`{"command":"echo hello"}`))
	if err != nil || result.IsError || !strings.Contains(strings.ToLower(result.Content), "hello") {
		t.Fatalf("command failed: result=%+v err=%v", result, err)
	}
	blockedCommand := "rm -rf /"
	if runtime.GOOS == "windows" {
		blockedCommand = "Stop-Computer"
	}
	blocked, err := enabled.CallTool(context.Background(), ToolBash, mustJSON(t, map[string]any{"command": blockedCommand}))
	if err != nil || !blocked.IsError || !strings.Contains(blocked.Content, "command_blocked") {
		t.Fatalf("dangerous command was not blocked: result=%+v err=%v", blocked, err)
	}
}

func TestCommandToolHonorsTimeout(t *testing.T) {
	client, err := New(Config{
		WorkDir:           t.TempDir(),
		MaxFileBytes:      1024,
		MaxOutputBytes:    1024,
		EnableCommandTool: true,
		CommandTimeout:    20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := "sleep 1"
	if runtime.GOOS == "windows" {
		command = "Start-Sleep -Seconds 1"
	}
	result, err := client.CallTool(context.Background(), ToolBash, mustJSON(t, map[string]any{"command": command}))
	if err != nil || !result.IsError || !strings.Contains(result.Content, "command_timeout") {
		t.Fatalf("timeout was not enforced: result=%+v err=%v", result, err)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
