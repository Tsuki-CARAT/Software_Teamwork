package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type AIGatewayConfig struct {
	BaseURL      string
	Model        string
	ProfileID    string
	Dimensions   int
	ServiceToken string
	HTTPClient   *http.Client
}

type AIGatewayClient struct {
	baseURL      string
	model        string
	profileID    string
	dimensions   int
	serviceToken string
	client       *http.Client
}

func NewAIGatewayClient(cfg AIGatewayConfig) (*AIGatewayClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("ai gateway base URL is required")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("ai gateway embedding model is required")
	}
	if cfg.Dimensions < 0 {
		return nil, fmt.Errorf("ai gateway embedding dimensions must be non-negative")
	}
	return &AIGatewayClient{
		baseURL:      baseURL,
		model:        model,
		profileID:    strings.TrimSpace(cfg.ProfileID),
		dimensions:   cfg.Dimensions,
		serviceToken: strings.TrimSpace(cfg.ServiceToken),
		client:       noRedirectHTTPClient(cfg.HTTPClient),
	}, nil
}

func noRedirectHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	} else {
		copied := *client
		client = &copied
	}
	// Embedding requests include service credentials and document chunks. Treat
	// redirects as dependency responses so neither headers nor body are replayed
	// to an unexpected host.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

func (c *AIGatewayClient) Embed(ctx context.Context, request service.EmbeddingRequest) (service.EmbeddingResult, error) {
	if err := ctx.Err(); err != nil {
		return service.EmbeddingResult{}, err
	}
	if len(request.Texts) == 0 {
		return service.EmbeddingResult{}, fmt.Errorf("embedding input must not be empty")
	}
	payload := embeddingRequest{
		ProfileID:      c.profileID,
		Model:          c.model,
		Input:          append([]string(nil), request.Texts...),
		Dimensions:     c.dimensions,
		EncodingFormat: "float",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return service.EmbeddingResult{}, fmt.Errorf("marshal ai gateway embedding request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return service.EmbeddingResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Caller-Service", "knowledge")
	if strings.TrimSpace(request.RequestID) != "" {
		httpReq.Header.Set("X-Request-Id", strings.TrimSpace(request.RequestID))
	}
	if strings.TrimSpace(request.UserID) != "" {
		httpReq.Header.Set("X-User-Id", strings.TrimSpace(request.UserID))
	}
	if c.serviceToken != "" {
		httpReq.Header.Set("X-Service-Token", c.serviceToken)
	}

	res, err := c.client.Do(httpReq)
	if err != nil {
		return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings returned HTTP %d", res.StatusCode)
	}

	var decoded embeddingResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&decoded); err != nil {
		return service.EmbeddingResult{}, fmt.Errorf("decode ai gateway embeddings response: %w", err)
	}
	if decoded.Object != "list" {
		return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings response object is invalid")
	}
	if len(decoded.Data) != len(request.Texts) {
		return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings response count mismatch")
	}
	sort.SliceStable(decoded.Data, func(i, j int) bool {
		return decoded.Data[i].Index < decoded.Data[j].Index
	})
	vectors := make([][]float32, len(decoded.Data))
	dimension := 0
	for index, item := range decoded.Data {
		if item.Index != index {
			return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings response index mismatch")
		}
		if len(item.Embedding) == 0 {
			return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings response vector is empty")
		}
		vector := make([]float32, len(item.Embedding))
		for i, value := range item.Embedding {
			vector[i] = float32(value)
		}
		if dimension == 0 {
			dimension = len(vector)
		} else if dimension != len(vector) {
			return service.EmbeddingResult{}, fmt.Errorf("ai gateway embeddings response dimension mismatch")
		}
		vectors[index] = vector
	}
	model := strings.TrimSpace(decoded.Model)
	if model == "" {
		model = c.model
	}
	return service.EmbeddingResult{
		Vectors:   vectors,
		Provider:  "ai_gateway",
		Model:     model,
		Dimension: dimension,
	}, nil
}

type embeddingRequest struct {
	ProfileID      string   `json:"profile_id,omitempty"`
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Object string            `json:"object"`
	Model  string            `json:"model"`
	Data   []embeddingVector `json:"data"`
}

type embeddingVector struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}
