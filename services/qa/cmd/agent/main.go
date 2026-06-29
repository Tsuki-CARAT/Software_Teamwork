package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/config"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/localtools"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/modelclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/toolclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

func main() {
	if err := run(); err != nil {
		slog.Error("qa agent stopped", "service", "qa", "operation", "agent_repl", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	local, err := localtools.New(localtools.Config{
		WorkDir:           cfg.WorkDir,
		MaxFileBytes:      cfg.MaxFileBytes,
		MaxOutputBytes:    cfg.MaxToolResultBytes,
		EnableCommandTool: cfg.EnableCommandTool,
		CommandTimeout:    cfg.CommandTimeout,
	})
	if err != nil {
		return err
	}
	providers := []agent.ToolClient{local}
	var remoteTools *mcpclient.Client
	if cfg.MCPTransport != config.TransportDisabled {
		remoteTools, err = mcpclient.Connect(ctx, mcpclient.Config{
			Transport:   cfg.MCPTransport,
			Command:     cfg.MCPServerCommand,
			Args:        cfg.MCPServerArgs,
			Endpoint:    cfg.MCPServerURL,
			Token:       cfg.MCPServerToken,
			TokenHeader: cfg.MCPServerTokenHeader,
		})
		if err != nil {
			return err
		}
		providers = append(providers, remoteTools)
		defer func() {
			if closeErr := remoteTools.Close(); closeErr != nil {
				slog.Warn("close MCP client", "service", "qa", "dependency", "mcp", "error", closeErr)
			}
		}()
	}
	tools, err := toolclient.New(providers...)
	if err != nil {
		return err
	}

	model, err := modelclient.New(modelclient.Config{
		Endpoint:    cfg.AIGatewayURL,
		Token:       cfg.AIGatewayToken,
		TokenHeader: cfg.AIGatewayTokenHeader,
		Model:       cfg.ModelID,
		MaxTokens:   cfg.MaxTokens,
		Timeout:     cfg.ModelTimeout,
	})
	if err != nil {
		return err
	}
	runner, err := agent.NewRunner(model, tools, agent.Config{
		MaxIterations:      cfg.MaxIterations,
		ToolTimeout:        cfg.MCPToolTimeout,
		MaxToolResultBytes: cfg.MaxToolResultBytes,
		Observer: func(event agent.Event) {
			fields := []any{"service", "qa", "operation", "agent_loop", "event", event.Type, "iteration", event.Iteration}
			if event.ToolName != "" {
				fields = append(fields, "tool", event.ToolName, "tool_call_id", event.ToolCallID)
			}
			if event.Err != nil {
				slog.Warn("agent event", append(fields, "status", "failed")...)
				return
			}
			slog.Debug("agent event", fields...)
		},
	})
	if err != nil {
		return err
	}

	history := []agent.Message{{Role: agent.RoleSystem, Content: cfg.SystemPrompt}}
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("QA MCP agent ready. Type 'exit' to quit.")
	for {
		fmt.Print("qa >> ")
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "exit" || line == "q" {
			return nil
		}
		if line != "" {
			input := append(append([]agent.Message(nil), history...), agent.Message{Role: agent.RoleUser, Content: line})
			result, runErr := runner.Run(ctx, input)
			if runErr != nil {
				if errors.Is(runErr, context.Canceled) {
					return nil
				}
				slog.Error("agent run failed", "service", "qa", "operation", "agent_loop", "error", runErr)
			} else {
				history = result.Messages
				fmt.Println(result.Final.Content)
			}
		}
		if readErr != nil {
			return nil
		}
	}
}
