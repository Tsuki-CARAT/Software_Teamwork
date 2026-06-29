package aigatewayclient

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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

const defaultTimeout = 90 * time.Second

// ChatMessage is an OpenAI-compatible chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	ProfileID string        `json:"profile_id,omitempty"`
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// Client calls the AI Gateway internal chat completions API.
type Client struct {
	baseURL      string
	serviceToken string
	profileID    string
	model        string
	httpClient   *http.Client
}

func New(baseURL, serviceToken, profileID, model string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("AI Gateway URL must be an absolute http(s) URL")
	}
	if model == "" {
		model = "default"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:      strings.TrimRight(parsed.String(), "/"),
		serviceToken: serviceToken,
		profileID:    profileID,
		model:        model,
		httpClient:   httpClient,
	}, nil
}

// ChatCompletion sends messages to the AI Gateway and returns the assistant reply.
// requestID is forwarded as X-Request-Id for tracing.
func (c *Client) ChatCompletion(ctx context.Context, requestID string, messages []ChatMessage) (string, error) {
	body := chatRequest{
		ProfileID: c.profileID,
		Model:     c.model,
		Messages:  messages,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", service.NewError(service.CodeDependency, "ai gateway request failed", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Caller-Service", "document")
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", service.NewError(service.CodeDependency, "ai gateway unavailable", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", service.NewError(service.CodeDependency, "ai gateway returned error status", nil)
	}
	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", service.NewError(service.CodeDependency, "ai gateway returned invalid response", err)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", service.NewError(service.CodeDependency, "ai gateway returned empty response", nil)
	}
	return result.Choices[0].Message.Content, nil
}
