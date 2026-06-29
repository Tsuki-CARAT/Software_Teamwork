package embedding_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/embedding"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestAIGatewayClientCreatesEmbeddingsWithContextHeaders(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/embeddings" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req_embed" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		if got := r.Header.Get("X-Caller-Service"); got != "knowledge" {
			t.Fatalf("X-Caller-Service = %q", got)
		}
		if got := r.Header.Get("X-User-Id"); got != "usr_123" {
			t.Fatalf("X-User-Id = %q", got)
		}
		if got := r.Header.Get("X-Service-Token"); got != "svc_secret" {
			t.Fatalf("X-Service-Token = %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["profile_id"] != "mp_embedding_default" || payload["model"] != "bge-small" {
			t.Fatalf("payload = %+v", payload)
		}
		input, _ := payload["input"].([]any)
		if len(input) != 2 || input[0] != "alpha" || input[1] != "beta" {
			t.Fatalf("input = %+v", input)
		}
		if payload["encoding_format"] != "float" || payload["dimensions"].(float64) != 2 {
			t.Fatalf("payload = %+v", payload)
		}
		if _, exists := payload["apiKey"]; exists {
			t.Fatalf("payload leaked apiKey: %+v", payload)
		}

		body := `{
			"object":"list",
			"model":"bge-small",
			"data":[
				{"object":"embedding","index":1,"embedding":[0.3,0.4]},
				{"object":"embedding","index":0,"embedding":[0.1,0.2]}
			],
			"usage":{"prompt_tokens":4,"total_tokens":4}
		}`
		return jsonResponse(http.StatusOK, body), nil
	})

	client, err := embedding.NewAIGatewayClient(embedding.AIGatewayConfig{
		BaseURL:      "http://ai-gateway.test",
		Model:        "bge-small",
		ProfileID:    "mp_embedding_default",
		Dimensions:   2,
		ServiceToken: "svc_secret",
		HTTPClient:   &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewAIGatewayClient() error = %v", err)
	}
	result, err := client.Embed(context.Background(), service.EmbeddingRequest{
		Texts:     []string{"alpha", "beta"},
		RequestID: "req_embed",
		UserID:    "usr_123",
	})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if result.Provider != "ai_gateway" || result.Model != "bge-small" || result.Dimension != 2 {
		t.Fatalf("result metadata = %+v", result)
	}
	if len(result.Vectors) != 2 || result.Vectors[0][0] != 0.1 || result.Vectors[1][0] != 0.3 {
		t.Fatalf("vectors = %+v", result.Vectors)
	}
}

func TestAIGatewayClientDoesNotExposeSensitiveErrorBody(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadGateway, "provider failed api_key=secret object_key=internal/path raw document text"), nil
	})

	client, err := embedding.NewAIGatewayClient(embedding.AIGatewayConfig{
		BaseURL:    "http://ai-gateway.test",
		Model:      "bge-small",
		HTTPClient: &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewAIGatewayClient() error = %v", err)
	}
	_, err = client.Embed(context.Background(), service.EmbeddingRequest{Texts: []string{"alpha"}})
	if err == nil {
		t.Fatal("Embed() error = nil, want error")
	}
	for _, forbidden := range []string{"api_key", "object_key", "raw document text"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked %q: %v", forbidden, err)
		}
	}
}

func TestAIGatewayClientDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "redirect-target.test" {
			redirected = true
			return jsonResponse(http.StatusOK, `{"object":"list","data":[]}`), nil
		}
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header: http.Header{
				"Location": []string{"http://redirect-target.test/internal/v1/embeddings"},
			},
			Body: io.NopCloser(strings.NewReader("redirect")),
		}, nil
	})

	client, err := embedding.NewAIGatewayClient(embedding.AIGatewayConfig{
		BaseURL:      "http://ai-gateway.test",
		Model:        "bge-small",
		ServiceToken: "svc_secret",
		HTTPClient:   &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewAIGatewayClient() error = %v", err)
	}
	_, err = client.Embed(context.Background(), service.EmbeddingRequest{
		Texts:     []string{"private document chunk"},
		RequestID: "req_embed",
		UserID:    "usr_123",
	})
	if err == nil {
		t.Fatal("Embed() error = nil, want redirect response error")
	}
	if redirected {
		t.Fatal("embedding client followed redirect and risked forwarding token or document text")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
