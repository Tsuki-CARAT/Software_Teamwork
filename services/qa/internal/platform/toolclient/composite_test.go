package toolclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type fakeProvider struct {
	name   string
	called bool
}

func (f *fakeProvider) ListTools(context.Context) ([]agent.ToolDefinition, error) {
	return []agent.ToolDefinition{{Type: "function", Function: agent.FunctionTool{Name: f.name, Parameters: map[string]any{"type": "object"}}}}, nil
}

func (f *fakeProvider) CallTool(_ context.Context, name string, _ json.RawMessage) (agent.ToolResult, error) {
	f.called = true
	return agent.ToolResult{Content: name}, nil
}

func TestCompositeMergesAndRoutesProviders(t *testing.T) {
	local := &fakeProvider{name: "read_file"}
	mcp := &fakeProvider{name: "search_knowledge"}
	client, err := New(local, mcp)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	if _, err := client.CallTool(context.Background(), "search_knowledge", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if local.called || !mcp.called {
		t.Fatalf("wrong provider route: local=%v mcp=%v", local.called, mcp.called)
	}
}

func TestCompositeRejectsDuplicateToolNames(t *testing.T) {
	client, err := New(&fakeProvider{name: "read_file"}, &fakeProvider{name: "read_file"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListTools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "duplicate tool name") {
		t.Fatalf("unexpected duplicate result: %v", err)
	}
}
