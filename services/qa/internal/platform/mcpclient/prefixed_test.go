package mcpclient

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type prefixedFakeClient struct {
	calledName string
}

func (*prefixedFakeClient) ListTools(context.Context) ([]agent.ToolDefinition, error) {
	return []agent.ToolDefinition{{Type: "function", Function: agent.FunctionTool{Name: "search", Parameters: map[string]any{"type": "object"}}}}, nil
}

func (f *prefixedFakeClient) CallTool(_ context.Context, name string, _ json.RawMessage) (agent.ToolResult, error) {
	f.calledName = name
	return agent.ToolResult{Content: "ok"}, nil
}

func TestPrefixedRewritesToolNamesBothDirections(t *testing.T) {
	base := &prefixedFakeClient{}
	client, err := NewPrefixed("knowledge", base, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Function.Name != "knowledge__search" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	if _, err := client.CallTool(context.Background(), "knowledge__search", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if base.calledName != "search" {
		t.Fatalf("called tool = %q, want search", base.calledName)
	}
}

func TestPrefixedRejectsForeignTool(t *testing.T) {
	client, err := NewPrefixed("knowledge", &prefixedFakeClient{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CallTool(context.Background(), "report__search", nil); err == nil {
		t.Fatal("expected foreign tool to fail")
	}
}
