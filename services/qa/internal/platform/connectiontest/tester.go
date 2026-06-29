package connectiontest

import (
	"context"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/modelclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type Tester struct{}

func (Tester) TestLLM(ctx context.Context, config service.RuntimeLLMConfig) (service.LLMConnectionTestResult, error) {
	client, err := modelclient.New(modelclient.Config{
		Endpoint: config.Endpoint, Token: config.Token, TokenHeader: config.TokenHeader,
		Model: config.Model, ProfileID: config.ProfileID, MaxTokens: min(config.MaxTokens, 32), Timeout: config.Timeout,
	})
	if err != nil {
		return service.LLMConnectionTestResult{}, err
	}
	startedAt := time.Now()
	_, err = client.Complete(ctx, []agent.Message{{Role: agent.RoleUser, Content: "Reply with OK."}}, nil)
	if err != nil {
		return service.LLMConnectionTestResult{}, err
	}
	return service.LLMConnectionTestResult{Success: true, Model: config.Model, LatencyMS: time.Since(startedAt).Milliseconds()}, nil
}

func (Tester) TestMCP(ctx context.Context, config service.RuntimeMCPConfig) (service.MCPConnectionTestResult, error) {
	testCtx, cancel := context.WithTimeout(ctx, config.ToolTimeout)
	defer cancel()
	startedAt := time.Now()
	client, err := mcpclient.Connect(testCtx, mcpclient.Config{
		Transport: config.Transport, Command: config.Command, Args: config.Args,
		Endpoint: config.EndpointURL, Token: config.Token, TokenHeader: config.TokenHeader,
	})
	if err != nil {
		return service.MCPConnectionTestResult{}, err
	}
	defer client.Close()
	tools, err := client.ListTools(testCtx)
	if err != nil {
		return service.MCPConnectionTestResult{}, err
	}
	summaries := make([]service.ToolSummary, 0, len(tools))
	for _, tool := range tools {
		summaries = append(summaries, service.ToolSummary{Name: tool.Function.Name, Description: tool.Function.Description})
	}
	return service.MCPConnectionTestResult{
		Success: true, ToolCount: len(summaries), LatencyMS: time.Since(startedAt).Milliseconds(), Tools: summaries,
	}, nil
}
