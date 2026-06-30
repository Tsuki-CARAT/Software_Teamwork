package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	knowledgehttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestHealthReturnsEnvelope(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req_health")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	var body successBody
	decodeJSON(t, res.Body, &body)
	if body.RequestID != "req_health" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestKnowledgeBaseCreateListGetPatchDelete(t *testing.T) {
	server := newHTTPTestServer(t)

	createReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", strings.NewReader(`{"id":"kb_1","name":"规程库","docType":"REGULATION"}`), service.PermissionKnowledgeWrite)
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}
	var createBody knowledgeBaseResponse
	decodeJSON(t, createRes.Body, &createBody)
	if createBody.Data.ID != "kb_1" || createBody.Data.Name != "规程库" || createBody.Data.DocType != "REGULATION" {
		t.Fatalf("create body = %+v", createBody)
	}

	listReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases?page=1&pageSize=20", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRes.Code, listRes.Body.String())
	}
	var listBody knowledgeBaseListResponse
	decodeJSON(t, listRes.Body, &listBody)
	if listBody.Page.Total != 1 || len(listBody.Data) != 1 {
		t.Fatalf("list body = %+v", listBody)
	}

	patchReq := authorizedRequest(http.MethodPatch, "/internal/v1/knowledge-bases/kb_1", strings.NewReader(`{"description":"更新"}`), service.PermissionKnowledgeWrite)
	patchRes := httptest.NewRecorder()
	server.ServeHTTP(patchRes, patchReq)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", patchRes.Code, patchRes.Body.String())
	}
	var patchBody knowledgeBaseResponse
	decodeJSON(t, patchRes.Body, &patchBody)
	if patchBody.Data.Description != "更新" {
		t.Fatalf("patch description = %q", patchBody.Data.Description)
	}

	deleteReq := authorizedRequest(http.MethodDelete, "/internal/v1/knowledge-bases/kb_1", nil, service.PermissionKnowledgeWrite)
	deleteRes := httptest.NewRecorder()
	server.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteRes.Code, deleteRes.Body.String())
	}

	getReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_1", nil)
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, body = %s", getRes.Code, getRes.Body.String())
	}
}

func TestDocumentListAndDetail(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_test",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	contentType := "application/pdf"
	sizeBytes := int64(12)
	jobID := "job_1"
	repo.SeedDocument(service.KnowledgeDocument{
		ID:              "doc_1",
		KnowledgeBaseID: "kb_1",
		Name:            "规程.pdf",
		ContentType:     &contentType,
		SizeBytes:       &sizeBytes,
		Status:          service.DocumentStatusReady,
		ChunkCount:      5,
		Tags:            []string{"锅炉"},
		CurrentJobID:    &jobID,
		CreatedBy:       "usr_test",
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	server := knowledgehttp.NewServer(service.New(repo), knowledgehttp.Config{})

	listReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases/kb_1/documents?status=ready", nil)
	listRes := httptest.NewRecorder()
	server.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRes.Code, listRes.Body.String())
	}
	var listBody documentListResponse
	decodeJSON(t, listRes.Body, &listBody)
	if listBody.Page.Total != 1 || listBody.Data[0].ID != "doc_1" || listBody.Data[0].JobID != "job_1" {
		t.Fatalf("list body = %+v", listBody)
	}

	getReq := authorizedRequest(http.MethodGet, "/internal/v1/documents/doc_1", nil)
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRes.Code, getRes.Body.String())
	}
	var getBody documentResponse
	decodeJSON(t, getRes.Body, &getBody)
	if getBody.Data.ID != "doc_1" || getBody.Data.ChunkCount != 5 {
		t.Fatalf("get body = %+v", getBody)
	}
}

func TestBusinessRoutesRequireGatewayUser(t *testing.T) {
	server := newHTTPTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/knowledge-bases", nil)
	req.Header.Set("X-Request-Id", "req_no_user")
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
	var errBody errorResponseBody
	decodeJSON(t, res.Body, &errBody)
	if errBody.Error.Code != "unauthorized" {
		t.Fatalf("error code = %q", errBody.Error.Code)
	}
}

func TestWriteRoutesRequireKnowledgeWrite(t *testing.T) {
	server := newHTTPTestServer(t)
	req := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", strings.NewReader(`{"name":"规程库"}`))
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var errBody errorResponseBody
	decodeJSON(t, res.Body, &errBody)
	if errBody.Error.Code != "forbidden" {
		t.Fatalf("error code = %q", errBody.Error.Code)
	}
}

func TestInvalidBodyAndPagination(t *testing.T) {
	server := newHTTPTestServer(t)

	bodyReq := authorizedRequest(http.MethodPost, "/internal/v1/knowledge-bases", strings.NewReader(`{"name":"规程库","unknown":true}`), service.PermissionKnowledgeWrite)
	bodyRes := httptest.NewRecorder()
	server.ServeHTTP(bodyRes, bodyReq)
	if bodyRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid body status = %d", bodyRes.Code)
	}

	pageReq := authorizedRequest(http.MethodGet, "/internal/v1/knowledge-bases?page=abc", nil)
	pageRes := httptest.NewRecorder()
	server.ServeHTTP(pageRes, pageReq)
	if pageRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid page status = %d", pageRes.Code)
	}
}

func TestUploadDocumentReturnsPublicSummaryOnly(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 13, 0, 0, 0, time.UTC)
	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_test",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	seedHTTPParserConfig(repo, now)
	svc := service.NewWithDependencies(repo, &httpUploadFileClient{}, &httpUploadQueue{}, func() time.Time { return now }, func(prefix string) string {
		return prefix + "_test"
	})
	server := knowledgehttp.NewServer(svc, knowledgehttp.Config{ServiceVersion: "test", Environment: "test"})

	body, contentType := multipartBody(t, "knowledge-guide.pdf", "pdf-bytes", []string{"锅炉", "规程"})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_1/documents", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Request-Id", "req_upload")
	req.Header.Set("X-User-Id", "usr_test")
	req.Header.Set("X-User-Permissions", service.PermissionKnowledgeWrite)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, res.Body, &payload)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", payload["data"])
	}
	if _, exists := data["fileRef"]; exists {
		t.Fatal("fileRef must not be exposed")
	}
	if _, exists := data["fileId"]; exists {
		t.Fatal("fileId must not be exposed")
	}
	if data["id"] != "doc_test" || data["jobId"] != "job_test" || data["status"] != string(service.DocumentStatusUploaded) {
		t.Fatalf("data = %#v", data)
	}
	if got := data["name"]; got != "knowledge-guide.pdf" {
		t.Fatalf("name = %v", got)
	}
	gotTags, ok := data["tags"].([]any)
	if !ok || len(gotTags) != 2 || gotTags[0] != "锅炉" || gotTags[1] != "规程" {
		t.Fatalf("tags = %#v", data["tags"])
	}
}

func TestUploadDocumentRejectsMissingFile(t *testing.T) {
	server := newHTTPTestServer(t)
	body, contentType := multipartBodyWithoutFile(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_1/documents", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-User-Id", "usr_test")
	req.Header.Set("X-User-Permissions", service.PermissionKnowledgeWrite)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var errBody errorResponseBody
	decodeJSON(t, res.Body, &errBody)
	if errBody.Error.Code != "validation_error" || errBody.Error.Fields["file"] == "" {
		t.Fatalf("error = %+v", errBody.Error)
	}
}

func TestUploadDocumentAcceptsJSONTagsField(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 13, 30, 0, 0, time.UTC)
	repo.SeedKnowledgeBase(service.KnowledgeBase{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{}`),
		RetrievalStrategy: json.RawMessage(`{}`),
		CreatedBy:         "usr_test",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	seedHTTPParserConfig(repo, now)
	svc := service.NewWithDependencies(repo, &httpUploadFileClient{}, &httpUploadQueue{}, func() time.Time { return now }, func(prefix string) string {
		return prefix + "_test"
	})
	server := knowledgehttp.NewServer(svc, knowledgehttp.Config{ServiceVersion: "test", Environment: "test"})

	body, contentType := multipartBodyWithJSONTags(t, "knowledge-guide.pdf", "pdf-bytes", `["锅炉","规程"]`)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/knowledge-bases/kb_1/documents", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-User-Id", "usr_test")
	req.Header.Set("X-User-Permissions", service.PermissionKnowledgeWrite)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, res.Body, &payload)
	data := payload["data"].(map[string]any)
	gotTags := data["tags"].([]any)
	if len(gotTags) != 2 || gotTags[0] != "锅炉" || gotTags[1] != "规程" {
		t.Fatalf("tags = %#v", data["tags"])
	}
}

func newHTTPTestServer(t *testing.T) http.Handler {
	t.Helper()
	repo := repository.NewMemoryRepository()
	knowledge := service.NewWithOptions(repo, func() time.Time {
		return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	}, func(prefix string) string {
		return prefix + "_test"
	})
	return knowledgehttp.NewServer(knowledge, knowledgehttp.Config{ServiceVersion: "test", Environment: "test"})
}

func authorizedRequest(method string, target string, body *strings.Reader, permissions ...string) *http.Request {
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("X-Request-Id", "req_test")
	req.Header.Set("X-User-Id", "usr_test")
	if len(permissions) > 0 {
		req.Header.Set("X-User-Permissions", strings.Join(permissions, ","))
	}
	if method == http.MethodPost || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func decodeJSON(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("Decode() error = %v, body = %s", err, body.String())
	}
}

func multipartBody(t *testing.T, filename string, content string, tags []string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, tag := range tags {
		if err := writer.WriteField("tags", tag); err != nil {
			t.Fatalf("WriteField(tags) error = %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("write file body = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return &body, writer.FormDataContentType()
}

func multipartBodyWithoutFile(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("tags", "锅炉"); err != nil {
		t.Fatalf("WriteField(tags) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return &body, writer.FormDataContentType()
}

func multipartBodyWithJSONTags(t *testing.T, filename string, content string, tags string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("tags", tags); err != nil {
		t.Fatalf("WriteField(tags) error = %v", err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("write file body = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return &body, writer.FormDataContentType()
}

type successBody struct {
	Data      map[string]string `json:"data"`
	RequestID string            `json:"requestId"`
}

type pageBody struct {
	Page pageInfo `json:"page"`
}

type pageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type knowledgeBaseResponse struct {
	Data knowledgeBase `json:"data"`
}

type knowledgeBaseListResponse struct {
	Data []knowledgeBase `json:"data"`
	Page pageInfo        `json:"page"`
}

type knowledgeBase struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DocType     string `json:"docType"`
}

type documentResponse struct {
	Data document `json:"data"`
}

type documentListResponse struct {
	Data []document `json:"data"`
	Page pageInfo   `json:"page"`
}

type document struct {
	ID         string `json:"id"`
	JobID      string `json:"jobId"`
	ChunkCount int64  `json:"chunkCount"`
}

type errorResponseBody struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"requestId"`
		Fields    map[string]string `json:"fields"`
	} `json:"error"`
}

func seedHTTPParserConfig(repo *repository.MemoryRepository, now time.Time) {
	repo.SeedParserConfig(service.ParserConfig{
		ID:                    "parser_default",
		Name:                  "Default builtin parser",
		Backend:               service.ParserBackendBuiltin,
		Enabled:               true,
		IsDefault:             true,
		Concurrency:           4,
		SupportedContentTypes: []string{"application/pdf"},
		DefaultParameters:     json.RawMessage(`{}`),
		CreatedAt:             now,
		UpdatedAt:             now,
	})
}

type httpUploadFileClient struct{}

func (c *httpUploadFileClient) CreateFile(context.Context, service.RequestContext, service.UploadedFile) (service.FileObject, error) {
	return service.FileObject{
		ID:             "file_test",
		Filename:       "knowledge-guide.pdf",
		ContentType:    "application/pdf",
		SizeBytes:      9,
		ChecksumSHA256: "abc123",
		CreatedAt:      time.Date(2026, 6, 29, 13, 0, 0, 0, time.UTC),
	}, nil
}

func (c *httpUploadFileClient) DeleteFile(context.Context, service.RequestContext, string) error {
	return nil
}

type httpUploadQueue struct{}

func (q *httpUploadQueue) EnqueueDocumentIngestion(context.Context, service.DocumentIngestionTask) error {
	return nil
}
