package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

// mockJobSvc implements JobSvc for testing.
type mockJobSvc struct {
	createJobFn    func(ctx context.Context, input service.CreateJobInput) (service.ReportJob, error)
	listJobsFn     func(ctx context.Context, reportID string) ([]service.ReportJob, error)
	getJobFn       func(ctx context.Context, id string) (service.ReportJob, error)
	retryJobFn     func(ctx context.Context, id string) (service.ReportJobAttempt, error)
	listAttemptsFn func(ctx context.Context, jobID string) ([]service.ReportJobAttempt, error)
	listEventsFn   func(ctx context.Context, reportID string) ([]service.ReportEvent, error)
}

func (m *mockJobSvc) CreateJob(ctx context.Context, input service.CreateJobInput) (service.ReportJob, error) {
	return m.createJobFn(ctx, input)
}

func (m *mockJobSvc) ListJobs(ctx context.Context, reportID string) ([]service.ReportJob, error) {
	return m.listJobsFn(ctx, reportID)
}

func (m *mockJobSvc) GetJob(ctx context.Context, id string) (service.ReportJob, error) {
	return m.getJobFn(ctx, id)
}

func (m *mockJobSvc) RetryJob(ctx context.Context, id string) (service.ReportJobAttempt, error) {
	return m.retryJobFn(ctx, id)
}

func (m *mockJobSvc) ListAttempts(ctx context.Context, jobID string) ([]service.ReportJobAttempt, error) {
	return m.listAttemptsFn(ctx, jobID)
}

func (m *mockJobSvc) ListEvents(ctx context.Context, reportID string) ([]service.ReportEvent, error) {
	return m.listEventsFn(ctx, reportID)
}

func newTestServerWithJobSvc(svc JobSvc) *Server {
	return NewServer(Config{JobSvc: svc})
}

func TestListJobsEmptyList(t *testing.T) {
	mock := &mockJobSvc{
		listJobsFn: func(ctx context.Context, reportID string) ([]service.ReportJob, error) {
			return []service.ReportJob{}, nil
		},
	}
	server := newTestServerWithJobSvc(mock)

	req := httptest.NewRequest(http.MethodGet, "/reports/550e8400-e29b-41d4-a716-446655440000/jobs", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Data []jobResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 0 {
		t.Fatalf("expected empty list, got %d items", len(body.Data))
	}
}

func TestGetJobNotFound(t *testing.T) {
	mock := &mockJobSvc{
		getJobFn: func(ctx context.Context, id string) (service.ReportJob, error) {
			return service.ReportJob{}, service.NewError(service.CodeNotFound, "report job not found", nil)
		},
	}
	server := newTestServerWithJobSvc(mock)

	req := httptest.NewRequest(http.MethodGet, "/report-jobs/550e8400-e29b-41d4-a716-446655440001", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRetryJobMaxAttemptsReached(t *testing.T) {
	mock := &mockJobSvc{
		retryJobFn: func(ctx context.Context, id string) (service.ReportJobAttempt, error) {
			return service.ReportJobAttempt{}, service.NewError(service.CodeValidation, "max retry attempts reached", nil)
		},
	}
	server := newTestServerWithJobSvc(mock)

	req := httptest.NewRequest(http.MethodPost, "/report-jobs/550e8400-e29b-41d4-a716-446655440001/attempts", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListAttempts(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockJobSvc{
		listAttemptsFn: func(ctx context.Context, jobID string) ([]service.ReportJobAttempt, error) {
			return []service.ReportJobAttempt{
				{
					ID:            "attempt-1",
					JobID:         jobID,
					AttemptNumber: 1,
					TriggerSource: "system",
					Status:        service.JobStatusSucceeded,
					CreatedAt:     now,
				},
			}, nil
		},
	}
	server := newTestServerWithJobSvc(mock)

	req := httptest.NewRequest(http.MethodGet, "/report-jobs/550e8400-e29b-41d4-a716-446655440001/attempts", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListEvents(t *testing.T) {
	mock := &mockJobSvc{
		listEventsFn: func(ctx context.Context, reportID string) ([]service.ReportEvent, error) {
			return []service.ReportEvent{}, nil
		},
	}
	server := newTestServerWithJobSvc(mock)

	req := httptest.NewRequest(http.MethodGet, "/reports/550e8400-e29b-41d4-a716-446655440000/events", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
