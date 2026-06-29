package config

import (
	"testing"
	"time"
)

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AI_GATEWAY_URL", "")
	t.Setenv("AI_GATEWAY_TOKEN", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("MODEL_ID", "")
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("MCP_SERVER_COMMAND", "python")
	t.Setenv("MCP_SERVER_ARGS_JSON", `["server.py","--safe"]`)
}

func TestLoadStdioConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPTransport != TransportStdio || len(cfg.MCPServerArgs) != 2 {
		t.Fatalf("unexpected MCP config: %+v", cfg)
	}
	if cfg.ModelTimeout != 60*time.Second || cfg.MaxIterations != 8 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.HTTPAddr != ":8084" || cfg.ShutdownTimeout != 10*time.Second || cfg.MaxRequestBytes != 1<<20 {
		t.Fatalf("unexpected HTTP defaults: %+v", cfg)
	}
	if cfg.AIGatewayURL != "https://api.deepseek.com/chat/completions" || cfg.ModelID != "deepseek-v4-pro" {
		t.Fatalf("unexpected DeepSeek defaults: %+v", cfg)
	}
}

func TestLoadBuiltInToolsWithoutMCPServer(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_TRANSPORT", TransportDisabled)
	t.Setenv("MCP_SERVER_COMMAND", "")
	t.Setenv("AGENT_WORKDIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPTransport != TransportDisabled || cfg.EnableCommandTool {
		t.Fatalf("unexpected built-in tool defaults: %+v", cfg)
	}
}

func TestLoadDefaultsToBuiltInToolsOnly(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_SERVER_COMMAND", "")
	t.Setenv("AGENT_WORKDIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPTransport != TransportDisabled {
		t.Fatalf("MCP transport = %q, want disabled", cfg.MCPTransport)
	}
}

func TestLoadAcceptsFullDeepSeekEndpoint(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("DEEPSEEK_BASE_URL", "https://example.test/v1/chat/completions")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIGatewayURL != "https://example.test/v1/chat/completions" {
		t.Fatalf("endpoint = %q", cfg.AIGatewayURL)
	}
}

func TestLoadStreamableHTTPConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_TRANSPORT", TransportStreamableHTTP)
	t.Setenv("MCP_SERVER_URL", "https://mcp.example.test/mcp")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPServerURL != "https://mcp.example.test/mcp" {
		t.Fatalf("unexpected endpoint: %s", cfg.MCPServerURL)
	}
}

func TestLoadRejectsShellStyleArguments(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_SERVER_ARGS_JSON", `server.py --unsafe`)
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid JSON arguments to fail")
	}
}
