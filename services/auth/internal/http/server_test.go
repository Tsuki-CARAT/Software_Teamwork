package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authhttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/http"
)

func TestHealthReturnsEnvelope(t *testing.T) {
	server := authhttp.NewServer(authhttp.Config{ServiceVersion: "0.1.0"})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req_health")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	var body successBody
	decodeJSON(t, res.Body.Bytes(), &body)
	if body.RequestID != "req_health" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	if body.Data["service"] != "auth" || body.Data["status"] != "ok" {
		t.Fatalf("data = %+v", body.Data)
	}
}

func TestReadyWithoutDatabaseIsUnavailable(t *testing.T) {
	server := authhttp.NewServer(authhttp.Config{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set("X-Request-Id", "req_ready")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body readinessBody
	decodeJSON(t, res.Body.Bytes(), &body)
	if body.RequestID != "req_ready" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	if body.Data.Status != "not_ready" {
		t.Fatalf("status = %q", body.Data.Status)
	}
	if len(body.Data.Dependencies) != 1 || body.Data.Dependencies[0].Status != "not_configured" {
		t.Fatalf("dependencies = %+v", body.Data.Dependencies)
	}
}

func TestReadyWithHealthyDatabase(t *testing.T) {
	server := authhttp.NewServer(authhttp.Config{ReadinessChecker: fakeChecker{}})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body readinessBody
	decodeJSON(t, res.Body.Bytes(), &body)
	if body.Data.Status != "ready" || body.Data.Dependencies[0].Status != "ready" {
		t.Fatalf("body = %+v", body)
	}
}

func TestReadyWithFailedDatabase(t *testing.T) {
	server := authhttp.NewServer(authhttp.Config{ReadinessChecker: fakeChecker{err: errors.New("down")}})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", res.Code)
	}
	var body readinessBody
	decodeJSON(t, res.Body.Bytes(), &body)
	if body.Data.Dependencies[0].Status != "unavailable" {
		t.Fatalf("dependencies = %+v", body.Data.Dependencies)
	}
}

func TestNotFoundReturnsErrorEnvelope(t *testing.T) {
	server := authhttp.NewServer(authhttp.Config{})
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("X-Request-Id", "req_missing")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d", res.Code)
	}
	var body errorBody
	decodeJSON(t, res.Body.Bytes(), &body)
	if body.Error.Code != "not_found" || body.Error.RequestID != "req_missing" {
		t.Fatalf("error = %+v", body.Error)
	}
}

type fakeChecker struct {
	err error
}

func (c fakeChecker) Check(context.Context) error {
	return c.err
}

type successBody struct {
	Data      map[string]string `json:"data"`
	RequestID string            `json:"requestId"`
}

type readinessBody struct {
	Data struct {
		Status       string `json:"status"`
		Dependencies []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"dependencies"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}

type errorBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, body = %s", err, string(body))
	}
}
