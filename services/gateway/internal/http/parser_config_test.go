package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/service"
)

func TestParserConfigProxyUsesGatewayAuthAndSafeErrors(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_admin",
		UserID:      "usr_admin",
		Username:    "admin",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:parser-config:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	knowledge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Id") != "usr_admin" || r.Header.Get("X-Request-Id") != "req_proxy" {
			t.Fatalf("headers user=%q request=%q", r.Header.Get("X-User-Id"), r.Header.Get("X-Request-Id"))
		}
		if r.Header.Get("X-User-Roles") != "admin" {
			t.Fatalf("roles header = %q", r.Header.Get("X-User-Roles"))
		}
		if r.Header.Get("X-User-Permissions") != "admin:parser-config:write" {
			t.Fatalf("permissions header = %q", r.Header.Get("X-User-Permissions"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":{"code":"internal_error","message":"postgres at http://knowledge.internal: secret stack","requestId":"other"}}`)
	}))
	defer knowledge.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": knowledge.URL},
	})

	unauthorized := httptest.NewRecorder()
	server.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/admin/parser-configs", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/parser-configs", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-User-Id", "attacker")
	req.Header.Set("X-Request-Id", "req_proxy")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "knowledge.internal") || strings.Contains(res.Body.String(), "postgres") {
		t.Fatalf("leaked body=%s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "dependency_error") {
		t.Fatalf("body=%s", res.Body.String())
	}
}

func TestParserConfigProxyRejectsNonAdmin(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_user",
		UserID:      "usr_user",
		Username:    "user",
		Roles:       []string{"user"},
		Permissions: []string{"knowledge:read"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": "http://knowledge.invalid"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/parser-configs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestParserConfigProxyRejectsModelProfileOnlyPermission(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_model_admin",
		UserID:      "usr_model_admin",
		Username:    "model-admin",
		Roles:       []string{},
		Permissions: []string{"admin:model-profile:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": "http://knowledge.invalid"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/parser-configs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestParserConfigProxyPreservesConflict(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_parser_admin",
		UserID:      "usr_parser_admin",
		Username:    "parser-admin",
		Roles:       []string{},
		Permissions: []string{"admin:parser-config:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	knowledge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"error":{"code":"conflict","message":"resource already exists","requestId":"downstream"}}`)
	}))
	defer knowledge.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": knowledge.URL},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/parser-configs", strings.NewReader(`{"name":"Default builtin parser"}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_conflict")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "conflict" {
		t.Fatalf("error body = %+v", body.Error)
	}
}

func TestParserConfigProxyPreservesSafeValidationFields(t *testing.T) {
	hasher := testHasher(t)
	store := newMemorySessionStore()
	accessToken := "valid-token"
	store.putToken(t, hasher, accessToken, service.SessionCacheEntry{
		SessionID:   "sess_parser_admin",
		UserID:      "usr_parser_admin",
		Username:    "parser-admin",
		Roles:       []string{},
		Permissions: []string{"admin:parser-config:write"},
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
	})

	knowledge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"validation_error","message":"request validation failed","requestId":"downstream","fields":{"backend":"is not supported","endpointUrl":"must be an absolute URI","secret":"stack at http://knowledge.internal with token"}}}`)
	}))
	defer knowledge.Close()

	server := newGatewayTestServer(t, gatewayDeps{
		store:         store,
		hasher:        hasher,
		ownerBaseURLs: map[string]string{"knowledge": knowledge.URL},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/parser-configs", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Request-Id", "req_validation")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var body errorBody
	decodeJSON(t, res.Body, &body)
	if body.Error.Code != "validation_error" || body.Error.Fields["backend"] != "is not supported" || body.Error.Fields["endpointUrl"] == "" {
		t.Fatalf("error body = %+v", body.Error)
	}
	if _, ok := body.Error.Fields["secret"]; ok {
		t.Fatalf("sensitive field leaked: %+v", body.Error.Fields)
	}
}
