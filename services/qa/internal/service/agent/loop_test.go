package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type fakeModel struct {
	responses []Completion
	requests  [][]Message
}

func (f *fakeModel) Complete(_ context.Context, messages []Message, _ []ToolDefinition) (Completion, error) {
	f.requests = append(f.requests, append([]Message(nil), messages...))
	if len(f.responses) == 0 {
		return Completion{}, errors.New("unexpected model call")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

type fakeTools struct {
	definitions []ToolDefinition
	result      ToolResult
	err         error
	calls       []string
	arguments   []json.RawMessage
}

type blockingTools struct {
	definitions []ToolDefinition
}

func (b *blockingTools) ListTools(context.Context) ([]ToolDefinition, error) {
	return b.definitions, nil
}

func (b *blockingTools) CallTool(ctx context.Context, _ string, _ json.RawMessage) (ToolResult, error) {
	<-ctx.Done()
	return ToolResult{}, ctx.Err()
}

func (f *fakeTools) ListTools(context.Context) ([]ToolDefinition, error) {
	return f.definitions, nil
}

func (f *fakeTools) CallTool(_ context.Context, name string, arguments json.RawMessage) (ToolResult, error) {
	f.calls = append(f.calls, name)
	f.arguments = append(f.arguments, append(json.RawMessage(nil), arguments...))
	return f.result, f.err
}

func testRunner(t *testing.T, model ModelClient, tools ToolClient, maxIterations int) *Runner {
	t.Helper()
	runner, err := NewRunner(model, tools, Config{
		MaxIterations:      maxIterations,
		ToolTimeout:        time.Second,
		MaxToolResultBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func addToolDefinition() ToolDefinition {
	return ToolDefinition{Type: "function", Function: FunctionTool{
		Name:        "add",
		Description: "add two numbers",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
		},
	}}
}

func TestRunnerExecutesMCPToolAndContinues(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID: "call-1", Type: "function", Function: FunctionCall{Name: "add", Arguments: `{"a":1,"b":2}`},
		}}}, FinishReason: "tool_calls"},
		{Message: Message{Role: RoleAssistant, Content: "3"}, FinishReason: "stop"},
	}}
	tools := &fakeTools{definitions: []ToolDefinition{addToolDefinition()}, result: ToolResult{Content: `{"sum":3}`}}
	runner := testRunner(t, model, tools, 4)

	result, err := runner.Run(context.Background(), []Message{{Role: RoleUser, Content: "1+2?"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Final.Content != "3" || result.Iterations != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(tools.calls) != 1 || tools.calls[0] != "add" {
		t.Fatalf("unexpected tool calls: %v", tools.calls)
	}
	if len(model.requests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(model.requests))
	}
	last := model.requests[1][len(model.requests[1])-1]
	if last.Role != RoleTool || last.ToolCallID != "call-1" || last.Content != `{"sum":3}` {
		t.Fatalf("unexpected tool result message: %+v", last)
	}
}

func TestRunnerReturnsDirectModelAnswerWithoutTool(t *testing.T) {
	model := &fakeModel{responses: []Completion{{
		Message: Message{Role: RoleAssistant, Content: "hello"}, FinishReason: "stop",
	}}}
	tools := &fakeTools{}
	result, err := testRunner(t, model, tools, 2).Run(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Final.Content != "hello" || len(tools.calls) != 0 {
		t.Fatalf("unexpected direct answer: %+v, calls=%v", result, tools.calls)
	}
}

func TestRunnerReturnsUnknownToolToModel(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{
			ID: "call-1", Type: "function", Function: FunctionCall{Name: "delete_everything", Arguments: `{}`},
		}}}, FinishReason: "tool_calls"},
		{Message: Message{Role: RoleAssistant, Content: "I cannot do that."}, FinishReason: "stop"},
	}}
	tools := &fakeTools{definitions: []ToolDefinition{addToolDefinition()}}
	result, err := testRunner(t, model, tools, 3).Run(context.Background(), []Message{{Role: RoleUser, Content: "delete"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Final.Content != "I cannot do that." {
		t.Fatalf("unexpected final content: %q", result.Final.Content)
	}
	if len(tools.calls) != 0 {
		t.Fatalf("unknown tool was executed: %v", tools.calls)
	}
	toolResult := model.requests[1][len(model.requests[1])-1]
	if !strings.Contains(toolResult.Content, "unknown_tool") {
		t.Fatalf("missing stable unknown-tool error: %s", toolResult.Content)
	}
}

func TestRunnerExecutesAllToolCallsFromOneModelTurn(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "call-1", Type: "function", Function: FunctionCall{Name: "add", Arguments: `{"a":1}`}},
			{ID: "call-2", Type: "function", Function: FunctionCall{Name: "add", Arguments: `{"a":2}`}},
		}}, FinishReason: "tool_calls"},
		{Message: Message{Role: RoleAssistant, Content: "done"}, FinishReason: "stop"},
	}}
	tools := &fakeTools{definitions: []ToolDefinition{addToolDefinition()}, result: ToolResult{Content: `{}`}}
	_, err := testRunner(t, model, tools, 3).Run(context.Background(), []Message{{Role: RoleUser, Content: "twice"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.calls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(tools.calls))
	}
	request := model.requests[1]
	if request[len(request)-2].ToolCallID != "call-1" || request[len(request)-1].ToolCallID != "call-2" {
		t.Fatalf("tool results were not correlated in order: %+v", request)
	}
}

func TestRunnerStopsAtMaximumIterations(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call-1", Function: FunctionCall{Name: "add", Arguments: `{}`}}}}},
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call-2", Function: FunctionCall{Name: "add", Arguments: `{}`}}}}},
	}}
	tools := &fakeTools{definitions: []ToolDefinition{addToolDefinition()}, result: ToolResult{Content: `{}`}}
	result, err := testRunner(t, model, tools, 2).Run(context.Background(), []Message{{Role: RoleUser, Content: "loop"}})
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("error = %v, want ErrMaxIterations", err)
	}
	if result.Iterations != 2 || len(tools.calls) != 2 {
		t.Fatalf("unexpected bounded result: %+v, calls=%v", result, tools.calls)
	}
}

func TestRunnerConvertsToolFailureToToolMessage(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call-1", Function: FunctionCall{Name: "add", Arguments: `{}`}}}}},
		{Message: Message{Role: RoleAssistant, Content: "Tool failed safely."}},
	}}
	tools := &fakeTools{definitions: []ToolDefinition{addToolDefinition()}, err: errors.New("secret downstream detail")}
	_, err := testRunner(t, model, tools, 3).Run(context.Background(), []Message{{Role: RoleUser, Content: "add"}})
	if err != nil {
		t.Fatal(err)
	}
	toolResult := model.requests[1][len(model.requests[1])-1].Content
	if !strings.Contains(toolResult, "tool_execution_failed") || strings.Contains(toolResult, "secret") {
		t.Fatalf("tool error was not sanitized: %s", toolResult)
	}
}

func TestRunnerBoundsToolCallWithTimeout(t *testing.T) {
	model := &fakeModel{responses: []Completion{
		{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call-1", Function: FunctionCall{Name: "add", Arguments: `{}`}}}}},
		{Message: Message{Role: RoleAssistant, Content: "timed out safely"}},
	}}
	tools := &blockingTools{definitions: []ToolDefinition{addToolDefinition()}}
	runner, err := NewRunner(model, tools, Config{
		MaxIterations:      3,
		ToolTimeout:        10 * time.Millisecond,
		MaxToolResultBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background(), []Message{{Role: RoleUser, Content: "add"}}); err != nil {
		t.Fatal(err)
	}
	toolResult := model.requests[1][len(model.requests[1])-1].Content
	if !strings.Contains(toolResult, "tool_execution_failed") {
		t.Fatalf("timeout was not converted to a safe tool result: %s", toolResult)
	}
}

func TestTruncateUTF8(t *testing.T) {
	got := truncateUTF8("你好世界", 30)
	if got != "你好世界" {
		t.Fatalf("unexpected untruncated value: %q", got)
	}
	got = truncateUTF8(strings.Repeat("界", 30), 32)
	if !strings.HasSuffix(got, "[tool result truncated]") || !utf8.ValidString(got) {
		t.Fatalf("invalid truncated UTF-8: %q", got)
	}
}
