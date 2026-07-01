package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/mcppolicy"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/httpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

const (
	TransportStdio          = "stdio"
	TransportStreamableHTTP = "streamable_http"
)

type Config struct {
	Transport   string
	Command     string
	Args        []string
	Endpoint    string
	Token       string
	TokenHeader string

	// AllowTestStdio enables the package-owned stdio echo server used by
	// integration tests. Runtime configuration must use streamable_http.
	AllowTestStdio bool
}

type Client struct {
	session *mcp.ClientSession
}

func Connect(ctx context.Context, cfg Config) (*Client, error) {
	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "qa-agent", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize MCP session: %w", err)
	}
	return &Client{session: session}, nil
}

func buildTransport(cfg Config) (mcp.Transport, error) {
	switch cfg.Transport {
	case TransportStdio:
		command, err := buildStdioCommand(cfg.Command, cfg.Args, cfg.AllowTestStdio)
		if err != nil {
			return nil, err
		}
		// MCP reserves stdout for JSON-RPC. Child diagnostics belong on stderr.
		command.Stderr = os.Stderr
		return &mcp.CommandTransport{Command: command}, nil
	case TransportStreamableHTTP:
		if strings.TrimSpace(cfg.Endpoint) == "" {
			return nil, errors.New("MCP HTTP endpoint is required")
		}
		base := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		}
		client := &http.Client{Transport: httpclient.HeaderTransport{
			Base:   base,
			Header: cfg.TokenHeader,
			Token:  cfg.Token,
		}}
		return &mcp.StreamableClientTransport{
			Endpoint:   cfg.Endpoint,
			HTTPClient: client,
			MaxRetries: 2,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported MCP transport %q", cfg.Transport)
	}
}

func buildStdioCommand(command string, args []string, allowTestStdio bool) (*exec.Cmd, error) {
	if !allowTestStdio {
		return nil, errors.New("MCP stdio transport is reserved for package tests; use streamable_http for runtime MCP servers")
	}
	spec, err := mcppolicy.ValidateStdioCommand(command, args)
	if err != nil {
		return nil, err
	}
	switch spec {
	case mcppolicy.StdioCommandEchoTest:
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return nil, errors.New("resolve MCP stdio test command directory")
		}
		cmd := exec.Command("go", "run", "./testserver/cmd/echo")
		cmd.Dir = filepath.Dir(file)
		return cmd, nil
	default:
		return nil, errors.New("MCP stdio command is not allowlisted")
	}
}

func (c *Client) Close() error {
	if c == nil || c.session == nil {
		return nil
	}
	return c.session.Close()
}

func (c *Client) ListTools(ctx context.Context) ([]agent.ToolDefinition, error) {
	if c == nil || c.session == nil {
		return nil, errors.New("MCP client is not connected")
	}
	var tools []agent.ToolDefinition
	cursor := ""
	for {
		result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("MCP tools/list: %w", err)
		}
		for _, tool := range result.Tools {
			if tool == nil {
				continue
			}
			parameters := tool.InputSchema
			if parameters == nil {
				parameters = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, agent.ToolDefinition{
				Type: "function",
				Function: agent.FunctionTool{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  parameters,
				},
			})
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (agent.ToolResult, error) {
	if c == nil || c.session == nil {
		return agent.ToolResult{}, errors.New("MCP client is not connected")
	}
	var decoded map[string]any
	if len(arguments) == 0 {
		decoded = map[string]any{}
	} else if err := json.Unmarshal(arguments, &decoded); err != nil {
		return agent.ToolResult{}, fmt.Errorf("decode MCP tool arguments: %w", err)
	}
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: decoded})
	if err != nil {
		return agent.ToolResult{}, fmt.Errorf("MCP tools/call %q: %w", name, err)
	}
	content, err := normalizeResult(result)
	if err != nil {
		return agent.ToolResult{}, err
	}
	return agent.ToolResult{Content: content, IsError: result.IsError}, nil
}

func normalizeResult(result *mcp.CallToolResult) (string, error) {
	if result == nil {
		return "", errors.New("MCP server returned an empty tool result")
	}
	if result.StructuredContent != nil {
		payload, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", fmt.Errorf("encode structured MCP result: %w", err)
		}
		return string(payload), nil
	}
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		switch value := item.(type) {
		case *mcp.TextContent:
			parts = append(parts, value.Text)
		default:
			payload, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("encode MCP content: %w", err)
			}
			parts = append(parts, string(payload))
		}
	}
	if len(parts) == 0 {
		return "{}", nil
	}
	return strings.Join(parts, "\n"), nil
}
