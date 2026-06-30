package parser_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestServiceClientPostsDocumentAndContextHeaders(t *testing.T) {
	var captured *http.Request
	var payload struct {
		DocumentName string `json:"documentName"`
		ContentType  string `json:"contentType"`
		SizeBytes    int64  `json:"sizeBytes"`
		DataBase64   string `json:"dataBase64"`
	}
	client, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL:      "https://parser.internal",
		ServiceToken: "secret-token",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			captured = req.Clone(req.Context())
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode request body error = %v", err)
			}
			return jsonResponse(http.StatusOK, `{"data":{"content":"Breaker OCR","backend":"paddleocr"},"requestId":"req_123"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewServiceClient() error = %v", err)
	}

	result, err := client.Parse(context.Background(), service.ParseInput{
		Name:        "scan.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader([]byte("%PDF")),
		SizeBytes:   4,
		RequestID:   "req_123",
		UserID:      "usr_123",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if result.Content != "Breaker OCR" {
		t.Fatalf("result = %+v", result)
	}
	if captured.URL.String() != "https://parser.internal/internal/v1/parsed-documents" {
		t.Fatalf("url = %s", captured.URL.String())
	}
	if captured.Header.Get("X-Request-Id") != "req_123" ||
		captured.Header.Get("X-Caller-Service") != "knowledge" ||
		captured.Header.Get("X-User-Id") != "usr_123" ||
		captured.Header.Get("X-Service-Token") != "secret-token" {
		t.Fatalf("headers = %+v", captured.Header)
	}
	if payload.DocumentName != "scan.pdf" || payload.ContentType != "application/pdf" || payload.SizeBytes != 4 {
		t.Fatalf("payload = %+v", payload)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.DataBase64)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if !bytes.Equal(decoded, []byte("%PDF")) {
		t.Fatalf("decoded payload = %q", string(decoded))
	}
}

func TestServiceClientParseDelegatesWholeDocumentToParserService(t *testing.T) {
	var capturedPath string
	var payload struct {
		DocumentName string `json:"documentName"`
		ContentType  string `json:"contentType"`
		SizeBytes    int64  `json:"sizeBytes"`
		DataBase64   string `json:"dataBase64"`
	}
	client, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL: "https://parser.internal",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedPath = req.URL.Path
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode request body error = %v", err)
			}
			return jsonResponse(http.StatusOK, `{"data":{"content":"Remote DOCX text","title":"Remote Title","backend":"paddleocr"},"requestId":"req_123"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewServiceClient() error = %v", err)
	}

	parsed, err := client.Parse(context.Background(), service.ParseInput{
		Name:        "manual.docx",
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Body:        bytes.NewReader([]byte("not-a-zip-but-remote-parser-handles-it")),
		SizeBytes:   38,
		RequestID:   "req_123",
		UserID:      "usr_123",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if capturedPath != "/internal/v1/parsed-documents" {
		t.Fatalf("path = %s", capturedPath)
	}
	if payload.DocumentName != "manual.docx" || payload.SizeBytes != 38 {
		t.Fatalf("payload = %+v", payload)
	}
	if parsed.Content != "Remote DOCX text" || parsed.Title != "Remote Title" || parsed.Backend != "paddleocr" {
		t.Fatalf("parsed = %+v", parsed)
	}
}

func TestServiceClientSanitizesFailure(t *testing.T) {
	client, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL: "https://parser.internal/private-path",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadGateway, `{"error":"secret document text"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewServiceClient() error = %v", err)
	}

	_, err = client.Parse(context.Background(), service.ParseInput{
		Name:        "scan.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader([]byte("secret document text")),
	})
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if containsAny(err.Error(), "secret", "private-path", "scan.pdf") {
		t.Fatalf("error leaked sensitive detail: %v", err)
	}
}

func TestServiceClientClassifiesParserValidationFailure(t *testing.T) {
	client, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL: "https://parser.internal",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadRequest, `{"error":{"code":"validation_error","message":"raw secret content"}}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewServiceClient() error = %v", err)
	}

	_, err = client.Parse(context.Background(), service.ParseInput{
		Name:        "bad.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader([]byte("raw secret content")),
		SizeBytes:   18,
	})
	if err == nil {
		t.Fatal("Parse() error = nil, want validation error")
	}
	appErr, ok := service.Classify(err)
	if !ok || appErr.Code != service.CodeValidation {
		t.Fatalf("error = %#v, want validation error", err)
	}
	if containsAny(err.Error(), "secret", "bad.pdf") {
		t.Fatalf("error leaked sensitive detail: %v", err)
	}
}

func TestServiceClientDoesNotFollowRedirectWithServiceToken(t *testing.T) {
	requests := []*http.Request{}
	client, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL:      "https://parser.internal",
		ServiceToken: "secret-token",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Clone(req.Context()))
			if len(requests) == 1 {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{"https://evil.internal/steal"}},
					Body:       io.NopCloser(bytes.NewBufferString("redirect")),
					Request:    req,
				}, nil
			}
			return jsonResponse(http.StatusOK, `{"data":{"content":"redirected","backend":"paddleocr"},"requestId":"req_123"}`), nil
		})},
	})
	if err != nil {
		t.Fatalf("NewServiceClient() error = %v", err)
	}

	_, err = client.Parse(context.Background(), service.ParseInput{
		Name:        "scan.pdf",
		ContentType: "application/pdf",
		Body:        bytes.NewReader([]byte("%PDF")),
	})
	if err == nil {
		t.Fatal("Parse() error = nil, want redirect status error")
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want no redirected request", len(requests))
	}
	if containsAny(err.Error(), "secret", "evil", "scan.pdf") {
		t.Fatalf("error leaked sensitive detail: %v", err)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func containsAny(value string, forbidden ...string) bool {
	for _, item := range forbidden {
		if item != "" && bytes.Contains([]byte(value), []byte(item)) {
			return true
		}
	}
	return false
}
