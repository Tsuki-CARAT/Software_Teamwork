package localtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

const (
	ToolReadFile  = "read_file"
	ToolWriteFile = "write_file"
	ToolEditFile  = "edit_file"
	ToolBash      = "bash"
)

type Config struct {
	WorkDir           string
	MaxFileBytes      int
	MaxOutputBytes    int
	EnableCommandTool bool
	CommandTimeout    time.Duration
}

type Client struct {
	root              string
	maxFileBytes      int
	maxOutputBytes    int
	enableCommandTool bool
	commandTimeout    time.Duration
}

func New(cfg Config) (*Client, error) {
	if cfg.MaxFileBytes <= 0 || cfg.MaxOutputBytes <= 0 {
		return nil, errors.New("file and output limits must be positive")
	}
	if cfg.CommandTimeout <= 0 {
		return nil, errors.New("command timeout must be positive")
	}
	root, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve tool workspace: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve tool workspace symlinks: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("tool workspace must be an existing directory")
	}
	return &Client{
		root:              root,
		maxFileBytes:      cfg.MaxFileBytes,
		maxOutputBytes:    cfg.MaxOutputBytes,
		enableCommandTool: cfg.EnableCommandTool,
		commandTimeout:    cfg.CommandTimeout,
	}, nil
}

func (c *Client) ListTools(context.Context) ([]agent.ToolDefinition, error) {
	tools := []agent.ToolDefinition{
		functionTool(ToolReadFile, "Read a UTF-8 text file inside the configured workspace.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":  map[string]any{"type": "string", "description": "Workspace-relative file path."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "description": "Optional maximum number of lines."},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		}),
		functionTool(ToolWriteFile, "Write a UTF-8 text file inside the configured workspace, creating parent directories.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Workspace-relative file path."},
				"content": map[string]any{"type": "string", "description": "Complete file content."},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": false,
		}),
		functionTool(ToolEditFile, "Replace the first exact occurrence of text in a UTF-8 file inside the configured workspace.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "Workspace-relative file path."},
				"old_text": map[string]any{"type": "string", "description": "Exact text to replace."},
				"new_text": map[string]any{"type": "string", "description": "Replacement text."},
			},
			"required":             []string{"path", "old_text", "new_text"},
			"additionalProperties": false,
		}),
	}
	if c.enableCommandTool {
		tools = append(tools, functionTool(ToolBash, "Run a shell command inside the configured workspace. This tool is explicitly enabled and bounded by a timeout.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":         map[string]any{"type": "string"},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		}))
	}
	return tools, nil
}

func functionTool(name, description string, parameters map[string]any) agent.ToolDefinition {
	return agent.ToolDefinition{
		Type: "function",
		Function: agent.FunctionTool{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (agent.ToolResult, error) {
	switch name {
	case ToolReadFile:
		var input struct {
			Path  string `json:"path"`
			Limit int    `json:"limit"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return failure("invalid_arguments", err.Error()), nil
		}
		return c.readFile(input.Path, input.Limit)
	case ToolWriteFile:
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return failure("invalid_arguments", err.Error()), nil
		}
		return c.writeFile(input.Path, input.Content)
	case ToolEditFile:
		var input struct {
			Path    string `json:"path"`
			OldText string `json:"old_text"`
			NewText string `json:"new_text"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return failure("invalid_arguments", err.Error()), nil
		}
		return c.editFile(input.Path, input.OldText, input.NewText)
	case ToolBash:
		if !c.enableCommandTool {
			return failure("tool_disabled", "command execution is disabled"), nil
		}
		var input struct {
			Command        string `json:"command"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return failure("invalid_arguments", err.Error()), nil
		}
		return c.runCommand(ctx, input.Command, input.TimeoutSeconds)
	default:
		return agent.ToolResult{}, fmt.Errorf("local tool %q is not registered", name)
	}
}

func decodeArguments(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("arguments do not match the tool schema")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("arguments must contain one JSON object")
	}
	return nil
}

func (c *Client) readFile(rawPath string, limit int) (agent.ToolResult, error) {
	path, err := c.resolveExisting(rawPath)
	if err != nil {
		return failure("invalid_path", err.Error()), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return failure("read_failed", "file could not be opened"), nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return failure("read_failed", "path is not a regular file"), nil
	}
	data, truncatedBytes, err := readBounded(file, c.maxFileBytes)
	if err != nil {
		return failure("read_failed", "file could not be read"), nil
	}
	if truncatedBytes {
		var valid bool
		data, valid = trimPartialUTF8(data)
		if !valid {
			return failure("unsupported_encoding", "file is not valid UTF-8 text"), nil
		}
	}
	if !utf8.Valid(data) {
		return failure("unsupported_encoding", "file is not valid UTF-8 text"), nil
	}
	text := string(data)
	if limit < 0 {
		return failure("invalid_arguments", "limit must be positive"), nil
	}
	if limit > 0 {
		lines := strings.Split(text, "\n")
		if len(lines) > limit {
			text = strings.Join(lines[:limit], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-limit)
		}
	}
	if truncatedBytes {
		text += "\n...[file truncated]"
	}
	return agent.ToolResult{Content: text}, nil
}

func (c *Client) writeFile(rawPath, content string) (agent.ToolResult, error) {
	if len(content) > c.maxFileBytes {
		return failure("file_too_large", "content exceeds the configured file limit"), nil
	}
	path, err := c.resolveForWrite(rawPath)
	if err != nil {
		return failure("invalid_path", err.Error()), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return failure("write_failed", "parent directory could not be created"), nil
	}
	if _, err := c.resolveForWrite(rawPath); err != nil {
		return failure("invalid_path", err.Error()), nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return failure("write_failed", "file could not be written"), nil
	}
	return agent.ToolResult{Content: fmt.Sprintf("Wrote %d bytes to %s", len(content), filepath.ToSlash(rawPath))}, nil
}

func (c *Client) editFile(rawPath, oldText, newText string) (agent.ToolResult, error) {
	if oldText == "" {
		return failure("invalid_arguments", "old_text must not be empty"), nil
	}
	path, err := c.resolveExisting(rawPath)
	if err != nil {
		return failure("invalid_path", err.Error()), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return failure("edit_failed", "file could not be read"), nil
	}
	data, tooLarge, readErr := readBounded(file, c.maxFileBytes)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return failure("edit_failed", "file could not be read"), nil
	}
	if tooLarge || len(data)-len(oldText)+len(newText) > c.maxFileBytes {
		return failure("file_too_large", "edited file exceeds the configured file limit"), nil
	}
	if !utf8.Valid(data) {
		return failure("unsupported_encoding", "file is not valid UTF-8 text"), nil
	}
	content := string(data)
	if !strings.Contains(content, oldText) {
		return failure("text_not_found", "old_text was not found"), nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return failure("edit_failed", "path is not a regular file"), nil
	}
	if err := os.WriteFile(path, []byte(strings.Replace(content, oldText, newText, 1)), info.Mode().Perm()); err != nil {
		return failure("edit_failed", "file could not be written"), nil
	}
	return agent.ToolResult{Content: "Edited " + filepath.ToSlash(rawPath)}, nil
}

func (c *Client) resolveExisting(rawPath string) (string, error) {
	candidate, err := c.workspacePath(rawPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", errors.New("path does not exist")
	}
	if err := ensureInside(c.root, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func (c *Client) resolveForWrite(rawPath string) (string, error) {
	candidate, err := c.workspacePath(rawPath)
	if err != nil {
		return "", err
	}
	ancestor := filepath.Dir(candidate)
	for {
		_, statErr := os.Lstat(ancestor)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return "", errors.New("path could not be inspected")
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", errors.New("path has no existing workspace ancestor")
		}
		ancestor = next
	}
	resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", errors.New("path ancestor could not be resolved")
	}
	if err := ensureInside(c.root, resolvedAncestor); err != nil {
		return "", err
	}
	if _, err := os.Lstat(candidate); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil {
			return "", errors.New("existing path could not be resolved")
		}
		if err := ensureInside(c.root, resolved); err != nil {
			return "", err
		}
	}
	return candidate, nil
}

func (c *Client) workspacePath(rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" || filepath.IsAbs(rawPath) {
		return "", errors.New("path must be workspace-relative")
	}
	clean := filepath.Clean(rawPath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the workspace")
	}
	candidate := filepath.Join(c.root, clean)
	if err := ensureInside(c.root, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func ensureInside(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("path escapes the workspace")
	}
	return nil
}

func (c *Client) runCommand(ctx context.Context, command string, requestedSeconds int) (agent.ToolResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return failure("invalid_arguments", "command must not be empty"), nil
	}
	if dangerousCommand(command) {
		return failure("command_blocked", "command matched a blocked dangerous pattern"), nil
	}
	timeout := c.commandTimeout
	if requestedSeconds > 0 && time.Duration(requestedSeconds)*time.Second < timeout {
		timeout = time.Duration(requestedSeconds) * time.Second
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var process *exec.Cmd
	if runtime.GOOS == "windows" {
		process = exec.CommandContext(commandCtx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		process = exec.CommandContext(commandCtx, "/bin/sh", "-c", command)
	}
	process.Dir = c.root
	output := &limitedBuffer{limit: c.maxOutputBytes}
	process.Stdout = output
	process.Stderr = output
	err := process.Run()
	text := strings.TrimSpace(output.String())
	if text == "" {
		text = "(no output)"
	}
	if commandCtx.Err() != nil {
		return failure("command_timeout", "command exceeded its timeout"), nil
	}
	if err != nil {
		return agent.ToolResult{Content: text + "\n[command failed]", IsError: true}, nil
	}
	return agent.ToolResult{Content: text}, nil
}

func dangerousCommand(command string) bool {
	normalized := strings.ToLower(command)
	patterns := []string{
		"rm -rf /", "sudo ", "shutdown", "reboot", "> /dev/",
		"format c:", "stop-computer", "remove-item c:\\",
	}
	for _, pattern := range patterns {
		if strings.Contains(normalized, pattern) {
			return true
		}
	}
	return strings.ContainsRune(command, '\x00')
}

func readBounded(reader io.Reader, limit int) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) <= limit {
		return data, false, nil
	}
	return data[:limit], true, nil
}

func trimPartialUTF8(data []byte) ([]byte, bool) {
	if utf8.Valid(data) {
		return data, true
	}
	for removed := 1; removed <= 3 && removed <= len(data); removed++ {
		candidate := data[:len(data)-removed]
		if utf8.Valid(candidate) {
			return candidate, true
		}
	}
	return nil, false
}

func failure(code, message string) agent.ToolResult {
	payload, _ := json.Marshal(map[string]any{"error": map[string]string{"code": code, "message": message}})
	return agent.ToolResult{Content: string(payload), IsError: true}
}

type limitedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
			b.truncated = true
		}
		_, _ = b.buffer.Write(data)
	} else if len(data) > 0 {
		b.truncated = true
	}
	return original, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	value := strings.ToValidUTF8(b.buffer.String(), "�")
	if b.truncated {
		value += "\n...[command output truncated]"
	}
	return value
}
