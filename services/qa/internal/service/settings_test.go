package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type settingsRepositoryStub struct {
	activeLLM StoredLLMConfig
}

func (r *settingsRepositoryStub) GetActiveQAConfig(context.Context) (RetrievalSettings, []string, error) {
	return RetrievalSettings{TopK: 5, ScoreThreshold: 0.1, RerankTopN: 3}, []string{}, nil
}

func (r *settingsRepositoryStub) GetActiveQAConfigVersion(context.Context) (QAConfigVersion, error) {
	return QAConfigVersion{ID: "qa-config"}, nil
}

func (r *settingsRepositoryStub) CreateQAConfigVersion(context.Context, string, RetrievalSettings, []string) error {
	return nil
}

func (r *settingsRepositoryStub) GetActiveLLMConfig(context.Context) (StoredLLMConfig, error) {
	if r.activeLLM.Provider != "" {
		return r.activeLLM, nil
	}
	return StoredLLMConfig{}, NewError(CodeNotFound, "active LLM configuration not found", nil)
}

func (r *settingsRepositoryStub) GetActiveLLMConfigVersion(context.Context) (LLMConfigVersion, error) {
	return LLMConfigVersion{ID: "llm-config"}, nil
}

func (r *settingsRepositoryStub) CreateLLMConfigVersion(context.Context, string, StoredLLMConfig) error {
	return nil
}

func (r *settingsRepositoryStub) GetRuntimeSetting(context.Context, string) (string, error) {
	return "", NewError(CodeNotFound, "runtime setting not found", nil)
}

func (r *settingsRepositoryStub) UpsertRuntimeSetting(context.Context, string, string) error {
	return nil
}

func (r *settingsRepositoryStub) ListMCPServers(context.Context) ([]MCPServerRecord, error) {
	return nil, nil
}

func (r *settingsRepositoryStub) GetMCPServer(context.Context, string) (MCPServerRecord, error) {
	return MCPServerRecord{}, NewError(CodeNotFound, "MCP server not found", nil)
}

func (r *settingsRepositoryStub) CreateMCPServer(context.Context, MCPServerRecord) (MCPServerRecord, error) {
	return MCPServerRecord{}, nil
}

func (r *settingsRepositoryStub) UpdateMCPServer(context.Context, MCPServerRecord) (MCPServerRecord, error) {
	return MCPServerRecord{}, nil
}

func (r *settingsRepositoryStub) DeleteMCPServer(context.Context, string) error {
	return nil
}

func (r *settingsRepositoryStub) UpdateMCPConnectionStatus(context.Context, string, int, *time.Time, string) error {
	return nil
}

func (r *settingsRepositoryStub) WriteAuditLog(context.Context, AuditLog) error {
	return nil
}

type settingsCipherStub struct{}

func (settingsCipherStub) Encrypt(value string) ([]byte, error) {
	return []byte(value), nil
}

func (settingsCipherStub) Decrypt(value []byte) (string, error) {
	return string(value), nil
}

type settingsLLMTesterStub struct {
	called bool
	seen   RuntimeLLMConfig
}

func (t *settingsLLMTesterStub) TestLLM(_ context.Context, config RuntimeLLMConfig) (LLMConnectionTestResult, error) {
	t.called = true
	t.seen = config
	return LLMConnectionTestResult{Success: true, Model: config.Model}, nil
}

type settingsMCPTesterStub struct{}

func (settingsMCPTesterStub) TestMCP(context.Context, RuntimeMCPConfig) (MCPConnectionTestResult, error) {
	return MCPConnectionTestResult{Success: true}, nil
}

func TestValidateRuntimeMCPAllowsStreamableHTTP(t *testing.T) {
	err := validateRuntimeMCP(RuntimeMCPConfig{
		Alias:       "echo_test",
		Transport:   "streamable_http",
		EndpointURL: "https://mcp.example.test/mcp",
		TokenHeader: "Authorization",
		ToolTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("validateRuntimeMCP returned error: %v", err)
	}
}

func TestValidateRuntimeMCPRejectsStdioTransport(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{name: "old exact test spec", command: "go", args: []string{"run", "./testserver/cmd/echo"}},
		{name: "shell", command: "sh", args: []string{"-c", "echo unsafe"}},
		{name: "path", command: "/usr/bin/go", args: []string{"run", "./testserver/cmd/echo"}},
		{name: "wrong args", command: "go", args: []string{"run", "./other"}},
		{name: "unsafe args", command: "go", args: []string{"run", "./testserver/cmd/echo\n--flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRuntimeMCP(RuntimeMCPConfig{
				Alias:       "echo_test",
				Transport:   "stdio",
				Command:     tt.command,
				Args:        tt.args,
				TokenHeader: "Authorization",
				ToolTimeout: time.Second,
			})
			var appErr *AppError
			if !errors.As(err, &appErr) || appErr.Code != CodeValidation || appErr.Fields["transport"] == "" {
				t.Fatalf("expected transport validation error, got %v", err)
			}
		})
	}
}

func TestValidateRuntimeLLMRejectsDirectProviderEscape(t *testing.T) {
	err := validateRuntimeLLM(RuntimeLLMConfig{
		Endpoint:    "http://169.254.169.254/latest/meta-data",
		Token:       "token",
		TokenHeader: "Authorization",
		Model:       "deepseek-chat",
		Timeout:     time.Second,
		MaxTokens:   100,
	})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != CodeValidation || appErr.Fields["llm.apiEndpoint"] == "" {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
}

func TestTestLLMConnectionRejectsStoredDirectProviderEscape(t *testing.T) {
	tester := &settingsLLMTesterStub{}
	svc, err := NewConfigService(&settingsRepositoryStub{
		activeLLM: StoredLLMConfig{
			Provider:        "direct",
			APIEndpoint:     "http://169.254.169.254/latest/meta-data",
			APIKeyEncrypted: []byte("token"),
			TokenHeader:     "Authorization",
			Model:           "deepseek-chat",
			TimeoutSeconds:  30,
			MaxTokens:       100,
		},
	}, settingsCipherStub{}, BootstrapSettings{}, tester, settingsMCPTesterStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.TestLLMConnection(context.Background(), LLMConnectionTestInput{})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != CodeValidation || appErr.Fields["llm.apiEndpoint"] == "" {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
	if tester.called {
		t.Fatal("LLM tester was called for unsafe stored endpoint")
	}
}

func TestTestLLMConnectionUsesTrustedStoredEndpoint(t *testing.T) {
	tester := &settingsLLMTesterStub{}
	svc, err := NewConfigService(&settingsRepositoryStub{
		activeLLM: StoredLLMConfig{
			Provider:        "direct",
			APIEndpoint:     "http://ai-gateway:8086/internal/v1/chat/completions",
			APIKeyEncrypted: []byte("token"),
			TokenHeader:     "X-Service-Token",
			Model:           "deepseek-chat",
			TimeoutSeconds:  30,
			MaxTokens:       100,
		},
	}, settingsCipherStub{}, BootstrapSettings{}, tester, settingsMCPTesterStub{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.TestLLMConnection(context.Background(), LLMConnectionTestInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !tester.called || result.Model != "deepseek-chat" || tester.seen.Endpoint != "http://ai-gateway:8086/internal/v1/chat/completions" {
		t.Fatalf("unexpected tester state result=%+v seen=%+v called=%v", result, tester.seen, tester.called)
	}
}
