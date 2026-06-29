package testserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoInput struct {
	Text string `json:"text" jsonschema:"text to echo"`
}

type echoOutput struct {
	Text string `json:"text" jsonschema:"echoed text"`
}

// EchoServer returns an MCP server with a single echo tool for integration tests.
func EchoServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "echo-server", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo text"},
		func(_ context.Context, _ *mcp.CallToolRequest, input echoInput) (*mcp.CallToolResult, echoOutput, error) {
			return nil, echoOutput{Text: input.Text}, nil
		})
	return server
}

// StreamableHTTP starts a streamable HTTP MCP echo server backed by httptest.
// When token is non-empty, requests must carry it in tokenHeader (defaults to Authorization).
func StreamableHTTP(t *testing.T, token, tokenHeader string) *httptest.Server {
	t.Helper()
	if tokenHeader == "" {
		tokenHeader = "Authorization"
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return EchoServer() }, nil)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			want := token
			if tokenHeader == "Authorization" {
				want = "Bearer " + token
			}
			if got := r.Header.Get(tokenHeader); got != want {
				t.Errorf("%s = %q, want %q", tokenHeader, got, want)
			}
		}
		handler.ServeHTTP(w, r)
	}))
}
