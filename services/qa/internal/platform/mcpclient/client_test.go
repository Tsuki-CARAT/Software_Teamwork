package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoInput struct {
	Text string `json:"text" jsonschema:"text to echo"`
}

type echoOutput struct {
	Text string `json:"text" jsonschema:"echoed text"`
}

func echoServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "echo-server", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo text"},
		func(_ context.Context, _ *mcp.CallToolRequest, input echoInput) (*mcp.CallToolResult, echoOutput, error) {
			return nil, echoOutput{Text: input.Text}, nil
		})
	return server
}

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("QA_MCP_HELPER_PROCESS") != "1" {
		return
	}
	if err := echoServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
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
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return echoServer() }, nil)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mcp-token" {
			t.Errorf("Authorization = %q", got)
		}
		mcpHandler.ServeHTTP(w, r)
	}))
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
