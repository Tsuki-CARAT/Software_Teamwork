package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type QdrantConfig struct {
	BaseURL    string
	APIKey     string
	Collection string
	Dimension  int
	HTTPClient *http.Client
}

type QdrantClient struct {
	baseURL    string
	apiKey     string
	collection string
	client     *http.Client
}

func NewQdrantClient(cfg QdrantConfig) (*QdrantClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("qdrant URL must be an absolute http(s) URL")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("qdrant URL must not contain credentials")
	}
	collection := strings.TrimSpace(cfg.Collection)
	if collection == "" {
		return nil, fmt.Errorf("qdrant collection is required")
	}
	return &QdrantClient{
		baseURL:    strings.TrimRight(parsed.String(), "/"),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		collection: collection,
		client:     noRedirectHTTPClient(cfg.HTTPClient),
	}, nil
}

func noRedirectHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	} else {
		copied := *client
		client = &copied
	}
	// Qdrant requests may include api-key headers, vectors, and metadata. Do
	// not replay them to a redirect target; surface the 3xx as dependency error.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

func (c *QdrantClient) Upsert(ctx context.Context, points []service.VectorPoint) error {
	payload := qdrantUpsertRequest{Points: make([]qdrantPoint, 0, len(points))}
	for _, point := range points {
		payload.Points = append(payload.Points, qdrantPoint{
			ID:      point.ID,
			Vector:  append([]float32(nil), point.Vector...),
			Payload: cloneMap(point.Payload),
		})
	}
	return c.postJSON(ctx, http.MethodPut, "/collections/"+url.PathEscape(c.collection)+"/points?wait=true", payload)
}

func (c *QdrantClient) DeleteByDocument(ctx context.Context, documentID string) error {
	payload := qdrantDeleteRequest{
		Filter: qdrantFilter{Must: []qdrantCondition{{
			Key:   "document_id",
			Match: qdrantMatch{Value: strings.TrimSpace(documentID)},
		}}},
	}
	return c.postJSON(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/delete?wait=true", payload)
}

func (c *QdrantClient) postJSON(ctx context.Context, method string, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return service.NewError(service.CodeDependency, "qdrant request could not be encoded", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return service.NewError(service.CodeDependency, "qdrant request failed", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return service.NewError(service.CodeDependency, "qdrant unavailable", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		return service.NewError(service.CodeDependency, "qdrant request failed", nil)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
	return nil
}

type qdrantUpsertRequest struct {
	Points []qdrantPoint `json:"points"`
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type qdrantDeleteRequest struct {
	Filter qdrantFilter `json:"filter"`
}

type qdrantFilter struct {
	Must []qdrantCondition `json:"must"`
}

type qdrantCondition struct {
	Key   string      `json:"key"`
	Match qdrantMatch `json:"match"`
}

type qdrantMatch struct {
	Value string `json:"value"`
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
