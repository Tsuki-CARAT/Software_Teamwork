package config

import (
	"testing"
	"time"
)

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AI_GATEWAY_URL", "")
	t.Setenv("AI_GATEWAY_TOKEN", "")
	t.Setenv("AI_GATEWAY_TOKEN_HEADER", "")
	t.Setenv("AI_GATEWAY_PROFILE_ID", "")
	t.Setenv("AI_GATEWAY_STREAM", "")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "test-service-token")
	t.Setenv("MODEL_ID", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_SERVER_COMMAND", "")
	t.Setenv("MCP_SERVER_ARGS_JSON", "")
}

func TestLoadDefaultConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPTransport != TransportDisabled || len(cfg.MCPServerArgs) != 0 {
		t.Fatalf("unexpected MCP config: %+v", cfg)
	}
	if cfg.ModelTimeout != 60*time.Second || cfg.MaxIterations != 8 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.HTTPAddr != ":8084" || cfg.ShutdownTimeout != 10*time.Second || cfg.MaxRequestBytes != 1<<20 {
		t.Fatalf("unexpected HTTP defaults: %+v", cfg)
	}
	if cfg.AIGatewayURL != defaultAIGatewayURL ||
		cfg.AIGatewayToken != "test-service-token" ||
		cfg.AIGatewayTokenHeader != defaultAIGatewayTokenHeader ||
		cfg.ModelID != "deepseek-chat" ||
		cfg.AIGatewayStream {
		t.Fatalf("unexpected AI Gateway defaults: %+v", cfg)
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

func TestLoadAcceptsExplicitAIGatewayEndpoint(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("AI_GATEWAY_URL", "http://ai-gateway:8086/internal/v1/chat/completions")
	t.Setenv("AI_GATEWAY_TOKEN", "explicit-token")
	t.Setenv("AI_GATEWAY_TOKEN_HEADER", "X-Service-Token")
	t.Setenv("AI_GATEWAY_PROFILE_ID", "profile-chat")
	t.Setenv("AI_GATEWAY_STREAM", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AIGatewayURL != "http://ai-gateway:8086/internal/v1/chat/completions" ||
		cfg.AIGatewayToken != "explicit-token" ||
		cfg.AIGatewayTokenHeader != "X-Service-Token" ||
		cfg.AIGatewayProfileID != "profile-chat" ||
		!cfg.AIGatewayStream {
		t.Fatalf("unexpected AI Gateway override: %+v", cfg)
	}
}

func TestLoadRejectsUntrustedAIGatewayEndpoint(t *testing.T) {
	cases := []string{
		"https://public.example.test/internal/v1/chat/completions",
		"http://169.254.169.254/internal/v1/chat/completions",
		"http://10.0.0.5/internal/v1/chat/completions",
		"http://ai-gateway/internal/v1/model-profiles",
		"http://ai-gateway/internal/v1/chat/completions?redirect=http://example.test",
	}
	for _, endpoint := range cases {
		t.Run(endpoint, func(t *testing.T) {
			setRequiredEnvironment(t)
			t.Setenv("AI_GATEWAY_URL", endpoint)
			if _, err := Load(); err == nil {
				t.Fatalf("expected endpoint %q to fail", endpoint)
			}
		})
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

func TestLoadRejectsNonAllowlistedStdioSpec(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_TRANSPORT", TransportStdio)
	t.Setenv("MCP_SERVER_COMMAND", "python3")
	t.Setenv("MCP_SERVER_ARGS_JSON", `["server.py"]`)
	if _, err := Load(); err == nil {
		t.Fatal("expected non-allowlisted stdio command spec to fail")
	}
}

func TestLoadRejectsRuntimeStdioTransport(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("MCP_TRANSPORT", TransportStdio)
	t.Setenv("MCP_SERVER_COMMAND", "go")
	t.Setenv("MCP_SERVER_ARGS_JSON", `["run","./testserver/cmd/echo"]`)
	if _, err := Load(); err == nil {
		t.Fatal("expected runtime stdio transport to fail")
	}
}
