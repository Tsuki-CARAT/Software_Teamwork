package connectiontest

import (
	"context"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient/testserver"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func TestMCPConnectionStreamableHTTP(t *testing.T) {
	httpServer := testserver.StreamableHTTP(t, "mcp-token", "Authorization")
	defer httpServer.Close()

	result, err := (Tester{}).TestMCP(context.Background(), service.RuntimeMCPConfig{
		Transport:   mcpclient.TransportStreamableHTTP,
		EndpointURL: httpServer.URL,
		Token:       "mcp-token",
		TokenHeader: "Authorization",
		ToolTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("result = %+v, want success", result)
	}
	if result.ToolCount != 1 || len(result.Tools) != 1 || result.Tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", result)
	}
	if result.LatencyMS < 0 {
		t.Fatalf("latency = %d", result.LatencyMS)
	}
}
