package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const gatewayKnowledgeOwnerSmokeGate = "GATEWAY_KNOWLEDGE_OWNER_SMOKE"

func TestGatewayKnowledgeOwnerRouteSmoke(t *testing.T) {
	if os.Getenv(gatewayKnowledgeOwnerSmokeGate) != "1" {
		t.Skip("set GATEWAY_KNOWLEDGE_OWNER_SMOKE=1 to run the Gateway -> Knowledge owner route smoke")
	}

	cfg := loadGatewayOwnerSmokeConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	assertHTTPReady(t, ctx, "file", cfg.fileServiceBaseURL)
	assertHTTPReady(t, ctx, "parser", cfg.parserServiceBaseURL)
	assertPostgresReady(t, ctx, cfg.knowledgeDatabaseURL)
	assertRedisReady(t, ctx, cfg.redisAddr)
	assertHTTPReady(t, ctx, "knowledge", cfg.knowledgeServiceBaseURL)
	assertHTTPReady(t, ctx, "gateway", cfg.gatewayBaseURL)

	requestID := "req_gateway_knowledge_owner_smoke_" + safeIdentifierSuffix(newSmokeRunID(t))
	assertGatewayRejectsSpoofedKnowledgeContext(t, ctx, cfg, requestID+"_spoofed")
	accessToken := createGatewaySession(t, ctx, cfg, requestID)
	assertGatewayKnowledgeBases(t, ctx, cfg, accessToken, requestID)
}

type gatewayOwnerSmokeConfig struct {
	gatewayBaseURL          string
	fileServiceBaseURL      string
	parserServiceBaseURL    string
	knowledgeServiceBaseURL string
	knowledgeDatabaseURL    string
	redisAddr               string
	username                string
	password                string
}

func loadGatewayOwnerSmokeConfig(t *testing.T) gatewayOwnerSmokeConfig {
	t.Helper()

	required := map[string]string{
		"GATEWAY_BASE_URL":                               os.Getenv("GATEWAY_BASE_URL"),
		"FILE_SERVICE_BASE_URL":                          os.Getenv("FILE_SERVICE_BASE_URL"),
		"PARSER_SERVICE_BASE_URL":                        os.Getenv("PARSER_SERVICE_BASE_URL"),
		"KNOWLEDGE_SERVICE_BASE_URL":                     os.Getenv("KNOWLEDGE_SERVICE_BASE_URL"),
		"KNOWLEDGE_TEST_DATABASE_URL":                    os.Getenv("KNOWLEDGE_TEST_DATABASE_URL"),
		"KNOWLEDGE_REDIS_ADDR":                           firstNonEmptyEnv("KNOWLEDGE_REDIS_ADDR", "GATEWAY_REDIS_ADDR"),
		"GATEWAY_SMOKE_USERNAME or LOCAL_ADMIN_USERNAME": firstNonEmptyEnv("GATEWAY_SMOKE_USERNAME", "LOCAL_ADMIN_USERNAME"),
		"GATEWAY_SMOKE_PASSWORD or LOCAL_ADMIN_PASSWORD": firstNonEmptyEnv("GATEWAY_SMOKE_PASSWORD", "LOCAL_ADMIN_PASSWORD"),
	}
	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("GATEWAY_KNOWLEDGE_OWNER_SMOKE=1 requires %s", strings.Join(missing, ", "))
	}

	return gatewayOwnerSmokeConfig{
		gatewayBaseURL:          trimHTTPBaseURL(t, "GATEWAY_BASE_URL", required["GATEWAY_BASE_URL"]),
		fileServiceBaseURL:      trimHTTPBaseURL(t, "FILE_SERVICE_BASE_URL", required["FILE_SERVICE_BASE_URL"]),
		parserServiceBaseURL:    trimHTTPBaseURL(t, "PARSER_SERVICE_BASE_URL", required["PARSER_SERVICE_BASE_URL"]),
		knowledgeServiceBaseURL: trimHTTPBaseURL(t, "KNOWLEDGE_SERVICE_BASE_URL", required["KNOWLEDGE_SERVICE_BASE_URL"]),
		knowledgeDatabaseURL:    strings.TrimSpace(required["KNOWLEDGE_TEST_DATABASE_URL"]),
		redisAddr:               normalizeRedisAddr(t, required["KNOWLEDGE_REDIS_ADDR"]),
		username:                strings.TrimSpace(required["GATEWAY_SMOKE_USERNAME or LOCAL_ADMIN_USERNAME"]),
		password:                strings.TrimSpace(required["GATEWAY_SMOKE_PASSWORD or LOCAL_ADMIN_PASSWORD"]),
	}
}

func assertHTTPReady(t *testing.T, ctx context.Context, name string, baseURL string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/readyz", nil)
	if err != nil {
		t.Fatalf("build %s ready request: %v", name, err)
	}
	req.Header.Set("X-Request-Id", "req_gateway_knowledge_owner_precheck")
	res, err := smokeHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("%s readyz request failed: %v", name, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("%s readyz returned HTTP %d", name, res.StatusCode)
	}
}

func assertPostgresReady(t *testing.T, ctx context.Context, databaseURL string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect Knowledge PostgreSQL for owner smoke: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping Knowledge PostgreSQL for owner smoke: %v", err)
	}
}

func assertRedisReady(t *testing.T, ctx context.Context, addr string) {
	t.Helper()
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("connect Redis for owner smoke: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(conn, "PING\r\n"); err != nil {
		t.Fatalf("send Redis PING for owner smoke: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read Redis PING for owner smoke: %v", err)
	}
	if !strings.HasPrefix(line, "+PONG") {
		t.Fatalf("Redis PING response = %q, want PONG", strings.TrimSpace(line))
	}
}

func createGatewaySession(t *testing.T, ctx context.Context, cfg gatewayOwnerSmokeConfig, requestID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"username": cfg.username,
		"password": cfg.password,
	})
	if err != nil {
		t.Fatalf("encode gateway session request: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.gatewayBaseURL+"/api/v1/sessions", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build gateway session request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", requestID)

	res, err := smokeHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("gateway session request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		t.Fatalf("gateway session returned HTTP %d", res.StatusCode)
	}

	var decoded struct {
		Data struct {
			Session struct {
				AccessToken string `json:"accessToken"`
				TokenType   string `json:"tokenType"`
			} `json:"session"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded); err != nil {
		t.Fatalf("decode gateway session response: %v", err)
	}
	if strings.TrimSpace(decoded.RequestID) != requestID {
		t.Fatalf("gateway session requestId = %q, want %q", decoded.RequestID, requestID)
	}
	if strings.TrimSpace(decoded.Data.Session.AccessToken) == "" || !strings.EqualFold(decoded.Data.Session.TokenType, "Bearer") {
		t.Fatal("gateway session response did not include a bearer access token")
	}
	return strings.TrimSpace(decoded.Data.Session.AccessToken)
}

func assertGatewayRejectsSpoofedKnowledgeContext(t *testing.T, ctx context.Context, cfg gatewayOwnerSmokeConfig, requestID string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.gatewayBaseURL+"/api/v1/knowledge-bases?page=1&pageSize=5", nil)
	if err != nil {
		t.Fatalf("build spoofed gateway knowledge bases request: %v", err)
	}
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set("X-User-Id", "spoofed-user-must-not-authenticate")
	req.Header.Set("X-User-Roles", "admin")
	req.Header.Set("X-User-Permissions", "knowledge:read,knowledge:write")

	res, err := smokeHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("spoofed gateway knowledge bases request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		t.Fatalf("spoofed gateway knowledge bases returned HTTP %d, want 401", res.StatusCode)
	}
	var decoded struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded); err != nil {
		t.Fatalf("decode spoofed gateway knowledge bases response: %v", err)
	}
	if decoded.Error.Code != "unauthorized" || strings.TrimSpace(decoded.Error.RequestID) != requestID {
		t.Fatalf("spoofed gateway knowledge bases error = %+v, want unauthorized with request id %q", decoded.Error, requestID)
	}
}

func assertGatewayKnowledgeBases(t *testing.T, ctx context.Context, cfg gatewayOwnerSmokeConfig, accessToken string, requestID string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.gatewayBaseURL+"/api/v1/knowledge-bases?page=1&pageSize=5", nil)
	if err != nil {
		t.Fatalf("build gateway knowledge bases request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", requestID)
	req.Header.Set("X-User-Id", "spoofed-user-ignored-by-gateway")

	res, err := smokeHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("gateway knowledge bases request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		t.Fatalf("gateway knowledge bases returned HTTP %d", res.StatusCode)
	}

	var decoded struct {
		Data      []json.RawMessage `json:"data"`
		RequestID string            `json:"requestId"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded); err != nil {
		t.Fatalf("decode gateway knowledge bases response: %v", err)
	}
	if strings.TrimSpace(decoded.RequestID) != requestID {
		t.Fatalf("gateway knowledge bases requestId = %q, want %q", decoded.RequestID, requestID)
	}
	if decoded.Data == nil {
		t.Fatal("gateway knowledge bases response data is nil, want array")
	}
}

func trimHTTPBaseURL(t *testing.T, key string, raw string) string {
	t.Helper()
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		t.Fatalf("%s must be an absolute http(s) URL", key)
	}
	if parsed.User != nil {
		t.Fatalf("%s must not contain credentials", key)
	}
	return value
}

func normalizeRedisAddr(t *testing.T, raw string) string {
	t.Helper()
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "redis://") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" {
			t.Fatalf("KNOWLEDGE_REDIS_ADDR must be host:port or redis://host:port")
		}
		return parsed.Host
	}
	if _, _, err := net.SplitHostPort(value); err != nil {
		t.Fatalf("KNOWLEDGE_REDIS_ADDR must include host and port")
	}
	return value
}

func smokeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
