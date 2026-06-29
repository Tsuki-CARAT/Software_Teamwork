package vector_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestQdrantClientDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "redirect-target.test" {
			redirected = true
			if r.Header.Get("api-key") == "qdrant_secret" {
				t.Fatal("redirect target received qdrant api key")
			}
			return textResponse(http.StatusOK, "{}"), nil
		}
		return &http.Response{
			StatusCode: http.StatusPermanentRedirect,
			Header: http.Header{
				"Location": []string{"http://redirect-target.test/collections/kb_chunks/points?wait=true"},
			},
			Body: io.NopCloser(strings.NewReader("redirect")),
		}, nil
	})
	client, err := vector.NewQdrantClient(vector.QdrantConfig{
		BaseURL:    "http://qdrant.test",
		APIKey:     "qdrant_secret",
		Collection: "kb_chunks",
		HTTPClient: &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewQdrantClient() error = %v", err)
	}

	err = client.Upsert(context.Background(), []service.VectorPoint{{
		ID:     "point_1",
		Vector: []float32{0.1, 0.2},
		Payload: map[string]any{
			"knowledge_base_id": "kb_1",
			"document_id":       "doc_1",
			"chunk_id":          "chunk_1",
		},
	}})
	if err == nil {
		t.Fatal("Upsert() error = nil, want redirect response error")
	}
	var appErr *service.AppError
	if !errors.As(err, &appErr) || appErr.Code != service.CodeDependency {
		t.Fatalf("Upsert() error = %v, want dependency AppError", err)
	}
	if redirected {
		t.Fatal("qdrant client followed redirect and risked forwarding api key or vector payload")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
