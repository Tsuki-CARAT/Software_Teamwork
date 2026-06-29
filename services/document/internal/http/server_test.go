package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzUsesEnvelopeAndRequestID(t *testing.T) {
	server := NewServer(Config{
		ReadyChecker: readyCheckerFunc(func(context.Context) error { return nil }),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req_test")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Request-Id"); got != "req_test" {
		t.Fatalf("X-Request-Id = %q, want req_test", got)
	}

	var body struct {
		Data struct {
			Service string `json:"service"`
			Status  string `json:"status"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Service != "document" || body.Data.Status != "ok" || body.RequestID != "req_test" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func TestReadyzReportsDependencyFailure(t *testing.T) {
	server := NewServer(Config{
		ReadyChecker: readyCheckerFunc(func(context.Context) error {
			return errors.New("postgres unavailable")
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-Id", "req_ready")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "dependency_error" || body.Error.RequestID != "req_ready" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

type readyCheckerFunc func(context.Context) error

func (fn readyCheckerFunc) CheckReady(ctx context.Context) error {
	return fn(ctx)
}
