package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

type fakeReportFileService struct {
	files   []service.ReportFile
	content service.FileContent
	err     error
}

func (f *fakeReportFileService) ListReportFiles(context.Context, service.RequestContext, service.ReportFileListFilter) (service.ReportFileListResult, error) {
	return service.ReportFileListResult{
		Items: f.files,
		Page:  service.PageMeta{Page: 1, PageSize: 20, Total: len(f.files)},
	}, nil
}

func (f *fakeReportFileService) CreateReportFile(_ context.Context, rctx service.RequestContext, input service.CreateReportFileInput) (service.ReportFile, error) {
	return service.ReportFile{
		ID:        "rf-1",
		ReportID:  input.ReportID,
		JobID:     "job-1",
		Filename:  "report.docx",
		Format:    input.Format,
		Status:    service.ReportFileStatusPending,
		CreatedBy: rctx.UserID,
		CreatedAt: time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeReportFileService) GetReportFile(context.Context, service.RequestContext, string) (service.ReportFile, error) {
	if len(f.files) == 0 {
		return service.ReportFile{}, service.NewError(service.CodeNotFound, "report file not found", nil)
	}
	return f.files[0], nil
}

func (f *fakeReportFileService) ReadReportFileContent(context.Context, service.RequestContext, string) (service.FileContent, error) {
	if f.err != nil {
		return service.FileContent{}, f.err
	}
	return f.content, nil
}

func TestCreateReportFileReturnsAcceptedSafeDTO(t *testing.T) {
	server := NewServer(Config{ReportFileSvc: &fakeReportFileService{}})
	req := httptest.NewRequest(http.MethodPost, "/report-files", strings.NewReader(`{"reportId":"report-1","format":"docx"}`))
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "fileRef") || strings.Contains(body, "file_ref") || strings.Contains(body, "file-internal") {
		t.Fatalf("response leaked file internals: %s", body)
	}
	var envelope struct {
		Data struct {
			ID          string `json:"id"`
			ContentPath string `json:"contentPath"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != "rf-1" || envelope.Data.ContentPath != "/api/v1/report-files/rf-1/content" {
		t.Fatalf("unexpected response data: %+v", envelope.Data)
	}
}

func TestGetReportFileReturnsSafeDTO(t *testing.T) {
	now := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	server := NewServer(Config{ReportFileSvc: &fakeReportFileService{
		files: []service.ReportFile{{
			ID:        "rf-1",
			ReportID:  "report-1",
			JobID:     "job-1",
			FileRef:   "file_internal_report",
			Filename:  "report.docx",
			Format:    service.ReportFileFormatDOCX,
			FileSize:  128,
			Status:    service.ReportFileStatusSucceeded,
			CreatedBy: "user-1",
			CreatedAt: now,
		}},
	}})
	req := httptest.NewRequest(http.MethodGet, "/report-files/rf-1", nil)
	req.Header.Set("X-User-Id", "user-1")
	req.Header.Set("X-Request-Id", "req-file")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertNoDocumentInternals(t, rec.Body.String())
	var envelope struct {
		Data struct {
			ID          string `json:"id"`
			ContentPath string `json:"contentPath"`
			FileSize    int64  `json:"fileSize"`
		} `json:"data"`
		RequestID string `json:"requestId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.ID != "rf-1" || envelope.Data.FileSize != 128 || envelope.RequestID != "req-file" {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}

func TestGetReportFileContentStreamsBinary(t *testing.T) {
	server := NewServer(Config{ReportFileSvc: &fakeReportFileService{
		content: service.FileContent{
			Filename:    "report.docx",
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			SizeBytes:   10,
			Content:     io.NopCloser(strings.NewReader("docx-bytes")),
		},
	}})
	req := httptest.NewRequest(http.MethodGet, "/report-files/rf-1/content", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "docx-bytes" {
		t.Fatalf("body = %q", got)
	}
	if strings.Contains(rec.Body.String(), `"data"`) {
		t.Fatalf("binary content was wrapped as JSON: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "report.docx") {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestGetReportFileContentFailureUsesErrorEnvelope(t *testing.T) {
	server := NewServer(Config{ReportFileSvc: &fakeReportFileService{
		err: service.NewError(service.CodeDependency, "file service failed", errors.New("minio bucket raw object")),
	}})
	req := httptest.NewRequest(http.MethodGet, "/report-files/rf-1/content", nil)
	req.Header.Set("X-User-Id", "user-1")
	req.Header.Set("X-Request-Id", "req-file-error")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON error envelope", got)
	}
	assertNoDocumentInternals(t, rec.Body.String())
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Error.Code != "dependency_error" || envelope.Error.RequestID != "req-file-error" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}
