package mcpclient

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient/testserver"
)

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("QA_MCP_HELPER_PROCESS") != "1" {
		return
	}
	if err := testserver.EchoServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestStdioClientLifecycleAndToolCall(t *testing.T) {
	t.Setenv("QA_MCP_HELPER_PROCESS", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := Connect(ctx, Config{
		Transport: TransportStdio,
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestMCPHelperProcess"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	assertEchoClient(t, ctx, client)
}

func TestStreamableHTTPClientAddsTokenAndCallsTool(t *testing.T) {
	httpServer := testserver.StreamableHTTP(t, "mcp-token", "Authorization")
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := Connect(ctx, Config{
		Transport:   TransportStreamableHTTP,
		Endpoint:    httpServer.URL,
		Token:       "mcp-token",
		TokenHeader: "Authorization",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	assertEchoClient(t, ctx, client)
}

func assertEchoClient(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Function.Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	result, err := client.CallTool(ctx, "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || !strings.Contains(result.Content, "hello") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestNormalizeTextResult(t *testing.T) {
	got, err := normalizeResult(&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("normalizeResult = %q", got)
	}
}
