package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gatewayhttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/http"
)

func TestHealthReturnsEnvelopeAndRequestID(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req_health")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Request-Id"); got != "req_health" {
		t.Fatalf("X-Request-Id = %q", got)
	}
	var body healthBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != "req_health" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	if body.Data.Status != "ok" || body.Data.Service != "gateway" {
		t.Fatalf("health data = %+v", body.Data)
	}
}

func TestReadyReturnsEnvelopeAndGeneratedRequestID(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	headerID := res.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Fatal("missing X-Request-Id")
	}
	var body healthBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != headerID {
		t.Fatalf("requestId = %q, header = %q", body.RequestID, headerID)
	}
	if body.Data.Status != "ready" {
		t.Fatalf("status = %q", body.Data.Status)
	}
}

func TestReadyReturnsDependencyErrorWhenCheckFails(t *testing.T) {
	server := gatewayhttp.NewServer(gatewayhttp.Config{
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		ServiceVersion:     "test",
		Environment:        "test",
		RequestTimeout:     time.Second,
		MaxBodyBytes:       1024,
		CORSAllowedOrigins: []string{"*"},
		ReadyCheck: func(context.Context) error {
			return errors.New("redis unavailable")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-Id", "req_ready_down")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "dependency_error" || body.Error.RequestID != "req_ready_down" {
		t.Fatalf("error body = %+v", body.Error)
	}
}

func TestNotFoundReturnsErrorEnvelope(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("X-Request-Id", "req_missing")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "not_found" || body.Error.RequestID != "req_missing" {
		t.Fatalf("error body = %+v", body.Error)
	}
}

func TestCORSPreflight(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/knowledge-bases", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("missing Access-Control-Allow-Headers")
	}
}

func TestBodyLimitRejectsLargeRequest(t *testing.T) {
	server := gatewayhttp.NewServer(gatewayhttp.Config{
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		ServiceVersion:     "test",
		Environment:        "test",
		RequestTimeout:     time.Second,
		MaxBodyBytes:       4,
		CORSAllowedOrigins: []string{"*"},
	})
	req := httptest.NewRequest(http.MethodPost, "/missing", bytes.NewBufferString("12345"))
	req.Header.Set("X-Request-Id", "req_large")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.RequestID != "req_large" {
		t.Fatalf("error body = %+v", body.Error)
	}
}

func newHTTPTestServer(t *testing.T) http.Handler {
	t.Helper()
	return gatewayhttp.NewServer(gatewayhttp.Config{
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		ServiceVersion:     "test",
		Environment:        "test",
		RequestTimeout:     time.Second,
		MaxBodyBytes:       1024,
		CORSAllowedOrigins: []string{"*"},
	})
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
}

type healthBody struct {
	Data struct {
		Status      string `json:"status"`
		Service     string `json:"service"`
		Version     string `json:"version"`
		Environment string `json:"environment"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type errorBody struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"requestId"`
		Fields    map[string]string `json:"fields"`
	} `json:"error"`
}
