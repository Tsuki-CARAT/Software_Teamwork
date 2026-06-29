package toolclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

// Composite merges local and MCP tool clients behind the Agent Loop's single
// ToolClient boundary. Tool names must be globally unique.
type Composite struct {
	providers []agent.ToolClient
	mu        sync.RWMutex
	routes    map[string]agent.ToolClient
}

func New(providers ...agent.ToolClient) (*Composite, error) {
	filtered := make([]agent.ToolClient, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			return nil, errors.New("tool provider must not be nil")
		}
		filtered = append(filtered, provider)
	}
	if len(filtered) == 0 {
		return nil, errors.New("at least one tool provider is required")
	}
	return &Composite{providers: filtered}, nil
}

func (c *Composite) ListTools(ctx context.Context) ([]agent.ToolDefinition, error) {
	var definitions []agent.ToolDefinition
	routes := make(map[string]agent.ToolClient)
	for _, provider := range c.providers {
		tools, err := provider.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("list tool provider: %w", err)
		}
		for _, tool := range tools {
			name := tool.Function.Name
			if name == "" {
				return nil, errors.New("tool provider returned an empty tool name")
			}
			if _, exists := routes[name]; exists {
				return nil, fmt.Errorf("duplicate tool name %q", name)
			}
			routes[name] = provider
			definitions = append(definitions, tool)
		}
	}
	c.mu.Lock()
	c.routes = routes
	c.mu.Unlock()
	return definitions, nil
}

func (c *Composite) CallTool(ctx context.Context, name string, arguments json.RawMessage) (agent.ToolResult, error) {
	c.mu.RLock()
	provider := c.routes[name]
	c.mu.RUnlock()
	if provider == nil {
		return agent.ToolResult{}, fmt.Errorf("tool %q is not routed; call ListTools first", name)
	}
	return provider.CallTool(ctx, name, arguments)
}
