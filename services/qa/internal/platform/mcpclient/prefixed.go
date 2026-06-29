package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

var aliasPattern = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

// Prefixed exposes one MCP server's tools in the shared model namespace as
// <alias>__<tool>. Calls are translated back to the original MCP tool name.
type Prefixed struct {
	alias   string
	prefix  string
	client  agent.ToolClient
	timeout time.Duration
}

func NewPrefixed(alias string, client agent.ToolClient, timeout time.Duration) (*Prefixed, error) {
	if !aliasPattern.MatchString(alias) {
		return nil, errors.New("MCP alias must match ^[a-z0-9_]{2,32}$")
	}
	if client == nil {
		return nil, errors.New("MCP tool client is required")
	}
	if timeout <= 0 {
		return nil, errors.New("MCP tool timeout must be positive")
	}
	return &Prefixed{alias: alias, prefix: alias + "__", client: client, timeout: timeout}, nil
}

func (p *Prefixed) ListTools(ctx context.Context) ([]agent.ToolDefinition, error) {
	tools, err := p.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]agent.ToolDefinition, len(tools))
	for index, tool := range tools {
		if strings.TrimSpace(tool.Function.Name) == "" {
			return nil, errors.New("MCP server returned an empty tool name")
		}
		tool.Function.Name = p.prefix + tool.Function.Name
		result[index] = tool
	}
	return result, nil
}

func (p *Prefixed) CallTool(ctx context.Context, name string, arguments json.RawMessage) (agent.ToolResult, error) {
	if !strings.HasPrefix(name, p.prefix) {
		return agent.ToolResult{}, fmt.Errorf("tool %q does not belong to MCP alias %q", name, p.alias)
	}
	originalName := strings.TrimPrefix(name, p.prefix)
	if originalName == "" {
		return agent.ToolResult{}, errors.New("MCP tool name is empty after alias prefix")
	}
	toolCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	return p.client.CallTool(toolCtx, originalName, arguments)
}
