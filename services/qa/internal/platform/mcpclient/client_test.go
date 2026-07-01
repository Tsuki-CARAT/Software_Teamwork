package mcpclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient/testserver"
)

func TestStdioClientLifecycleAndToolCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := Connect(ctx, Config{
		Transport:      TransportStdio,
		Command:        "go",
		Args:           []string{"run", "./testserver/cmd/echo"},
		AllowTestStdio: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	assertEchoClient(t, ctx, client)
}

func TestStdioClientRejectsUnsafeCommand(t *testing.T) {
	_, err := buildTransport(Config{
		Transport: TransportStdio,
		Command:   "go",
		Args:      []string{"run", "./testserver/cmd/echo"},
	})
	if err == nil {
		t.Fatal("expected runtime stdio transport to be rejected")
	}
	_, err = buildTransport(Config{
		Transport:      TransportStdio,
		Command:        "python -c",
		Args:           []string{"print('unsafe')"},
		AllowTestStdio: true,
	})
	if err == nil {
		t.Fatal("expected unsafe command to be rejected")
	}
	_, err = buildTransport(Config{
		Transport:      TransportStdio,
		Command:        "sh",
		Args:           []string{"-c", "echo pwned"},
		AllowTestStdio: true,
	})
	if err == nil {
		t.Fatal("expected non-allowlisted command to be rejected")
	}
	_, err = buildTransport(Config{
		Transport:      TransportStdio,
		Command:        "go",
		Args:           []string{"run", "server.go\n--flag"},
		AllowTestStdio: true,
	})
	if err == nil {
		t.Fatal("expected unsafe argument to be rejected")
	}
	_, err = buildTransport(Config{
		Transport:      TransportStdio,
		Command:        "python3",
		Args:           []string{"server.py"},
		AllowTestStdio: true,
	})
	if err == nil {
		t.Fatal("expected non-allowlisted command spec to be rejected")
	}
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
