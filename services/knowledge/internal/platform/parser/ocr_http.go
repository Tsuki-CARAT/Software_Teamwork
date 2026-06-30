package parser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type ServiceClientConfig struct {
	BaseURL       string
	ServiceToken  string
	CallerService string
	Timeout       time.Duration
	Client        *http.Client
}

type ServiceClient struct {
	baseURL       string
	serviceToken  string
	callerService string
	client        *http.Client
}

const maxParserPayloadBytes = 8 << 20

func NewServiceClient(cfg ServiceClientConfig) (*ServiceClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("parser service base URL is required")
	}
	caller := strings.TrimSpace(cfg.CallerService)
	if caller == "" {
		caller = "knowledge"
	}
	client := cfg.Client
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	} else {
		copied := *client
		client = &copied
	}
	// Parser requests may include service credentials and document bytes. Treat
	// redirects as an error response so custom headers cannot be forwarded to
	// another host.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &ServiceClient{
		baseURL:       baseURL,
		serviceToken:  strings.TrimSpace(cfg.ServiceToken),
		callerService: caller,
		client:        client,
	}, nil
}

func (c *ServiceClient) Parse(ctx context.Context, input service.ParseInput) (service.ParsedDocument, error) {
	if input.Body == nil {
		return service.ParsedDocument{}, fmt.Errorf("document body is required")
	}
	limited := io.LimitReader(input.Body, maxParserPayloadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return service.ParsedDocument{}, err
	}
	if len(data) > maxParserPayloadBytes {
		return service.ParsedDocument{}, fmt.Errorf("document is too large for parser")
	}
	parsed, err := c.parseBytes(ctx, parserRequest{
		DocumentName: strings.TrimSpace(input.Name),
		ContentType:  strings.TrimSpace(input.ContentType),
		SizeBytes:    input.SizeBytes,
		DataBase64:   base64.StdEncoding.EncodeToString(data),
	}, input.RequestID, input.UserID)
	if err != nil {
		if _, ok := service.Classify(err); ok {
			return service.ParsedDocument{}, err
		}
		return service.ParsedDocument{}, service.DependencyError("document parser service failed", err)
	}
	content := strings.TrimSpace(parsed.Content)
	if content == "" {
		return service.ParsedDocument{}, fmt.Errorf("document is empty")
	}
	return service.ParsedDocument{
		Content: content,
		Title:   strings.TrimSpace(parsed.Title),
		Backend: strings.TrimSpace(parsed.Backend),
	}, nil
}

func (c *ServiceClient) parseBytes(ctx context.Context, payload parserRequest, requestID string, userID string) (parsedDocument, error) {
	payload = parserRequest{
		DocumentName: strings.TrimSpace(payload.DocumentName),
		ContentType:  strings.TrimSpace(payload.ContentType),
		SizeBytes:    payload.SizeBytes,
		DataBase64:   payload.DataBase64,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return parsedDocument{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/parsed-documents", bytes.NewReader(body))
	if err != nil {
		return parsedDocument{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Caller-Service", c.callerService)
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set("X-Request-Id", strings.TrimSpace(requestID))
	}
	if strings.TrimSpace(userID) != "" {
		req.Header.Set("X-User-Id", strings.TrimSpace(userID))
	}
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return parsedDocument{}, fmt.Errorf("parser service request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		switch res.StatusCode {
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
			return parsedDocument{}, service.ValidationError("document parsing failed", map[string]string{"file": "could not be parsed"})
		default:
			return parsedDocument{}, service.DependencyError("parser service failed", nil)
		}
	}
	var decoded parserResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, maxParserPayloadBytes+1)).Decode(&decoded); err != nil {
		return parsedDocument{}, fmt.Errorf("parser service response could not be decoded")
	}
	if len(decoded.Data.Content) > maxParserPayloadBytes {
		return parsedDocument{}, fmt.Errorf("parser service response is too large")
	}
	return decoded.Data, nil
}

type parserRequest struct {
	DocumentName string `json:"documentName,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	DataBase64   string `json:"dataBase64"`
}

type parserResponse struct {
	Data parsedDocument `json:"data"`
}

type parsedDocument struct {
	Content string `json:"content"`
	Title   string `json:"title"`
	Backend string `json:"backend"`
}
