package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

// fakeReportService implements ReportService for HTTP-layer tests so they
// don't depend on PostgreSQL.
type fakeReportService struct {
	reports map[string]service.Report
}

func newFakeReportService() *fakeReportService {
	return &fakeReportService{reports: map[string]service.Report{}}
}

func (f *fakeReportService) ListReports(_ context.Context, reqCtx service.RequestContext, filter service.ReportListFilter) (service.ReportListResult, error) {
	var result []service.Report
	for _, report := range f.reports {
		if !reqCtx.IsAdmin() && report.CreatorID != reqCtx.UserID {
			continue
		}
		result = append(result, report)
	}
	return service.ReportListResult{
		Items: result,
		Page:  service.PageMeta{Page: 1, PageSize: 20, Total: len(result)},
	}, nil
}

func (f *fakeReportService) CreateReport(_ context.Context, reqCtx service.RequestContext, input service.CreateReportInput) (service.Report, error) {
	if input.Name == "" {
		return service.Report{}, service.ValidationError(map[string]string{"name": "required"})
	}
	report := service.Report{
		ID:         "report-1",
		Name:       input.Name,
		ReportType: input.ReportType,
		TemplateID: input.TemplateID,
		Topic:      input.Topic,
		Status:     service.ReportStatusDraft,
		CreatorID:  reqCtx.UserID,
	}
	f.reports[report.ID] = report
	return report, nil
}

func (f *fakeReportService) GetReport(_ context.Context, reqCtx service.RequestContext, reportID string) (service.Report, error) {
	report, ok := f.reports[reportID]
	if !ok {
		return service.Report{}, service.NewError(service.CodeNotFound, "report not found", nil)
	}
	if !reqCtx.CanAccessReport(report) {
		return service.Report{}, service.NewError(service.CodeForbidden, "forbidden", nil)
	}
	return report, nil
}

func (f *fakeReportService) UpdateReport(context.Context, service.RequestContext, string, service.UpdateReportInput) (service.Report, error) {
	return service.Report{}, nil
}

func (f *fakeReportService) SoftDeleteReport(_ context.Context, reqCtx service.RequestContext, reportID string) error {
	report, ok := f.reports[reportID]
	if !ok {
		return service.NewError(service.CodeNotFound, "report not found", nil)
	}
	if !reqCtx.CanAccessReport(report) {
		return service.NewError(service.CodeForbidden, "forbidden", nil)
	}
	delete(f.reports, reportID)
	return nil
}

func (f *fakeReportService) ListOutlines(context.Context, service.RequestContext, string) ([]service.ReportOutline, error) {
	return nil, nil
}
func (f *fakeReportService) CreateOutline(context.Context, service.RequestContext, string, service.CreateOutlineInput) (service.ReportOutline, error) {
	return service.ReportOutline{}, nil
}
func (f *fakeReportService) GetOutline(context.Context, service.RequestContext, string, string) (service.ReportOutline, error) {
	return service.ReportOutline{}, nil
}
func (f *fakeReportService) UpdateOutline(context.Context, service.RequestContext, string, string, service.UpdateOutlineInput) (service.ReportOutline, error) {
	return service.ReportOutline{}, nil
}
func (f *fakeReportService) DeleteOutlineSection(context.Context, service.RequestContext, string, string, string) (service.ReportOutline, error) {
	return service.ReportOutline{}, nil
}
func (f *fakeReportService) ListSections(context.Context, service.RequestContext, string) ([]service.ReportSection, error) {
	return nil, nil
}
func (f *fakeReportService) CreateSection(context.Context, service.RequestContext, string, service.CreateSectionInput) (service.ReportSection, error) {
	return service.ReportSection{}, nil
}
func (f *fakeReportService) GetSection(context.Context, service.RequestContext, string, string) (service.ReportSection, error) {
	return service.ReportSection{}, nil
}
func (f *fakeReportService) UpdateSection(context.Context, service.RequestContext, string, string, service.UpdateSectionInput) (service.ReportSection, error) {
	return service.ReportSection{}, nil
}
func (f *fakeReportService) ListSectionVersions(context.Context, service.RequestContext, string, string) ([]service.ReportSectionVersion, error) {
	return nil, nil
}
func (f *fakeReportService) CreateSectionVersion(context.Context, service.RequestContext, string, string, service.CreateSectionVersionInput) (service.ReportSectionVersion, error) {
	return service.ReportSectionVersion{}, nil
}

func TestCreateReportThenGetByOwner(t *testing.T) {
	fake := newFakeReportService()
	server := NewServer(Config{ReportService: fake})

	body := strings.NewReader(`{"name":"June report","reportType":"summer_peak_inspection","templateId":"tpl-1","topic":"summer"}`)
	req := httptest.NewRequest(http.MethodPost, "/reports", body)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/reports/report-1", nil)
	getReq.SetPathValue("reportId", "report-1")
	getReq.Header.Set("X-User-Id", "user-1")
	getRec := httptest.NewRecorder()
	server.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
}

func TestGetReportForbiddenForNonOwner(t *testing.T) {
	fake := newFakeReportService()
	fake.reports["report-1"] = service.Report{ID: "report-1", CreatorID: "owner-1"}
	server := NewServer(Config{ReportService: fake})

	req := httptest.NewRequest(http.MethodGet, "/reports/report-1", nil)
	req.SetPathValue("reportId", "report-1")
	req.Header.Set("X-User-Id", "intruder")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != string(service.CodeForbidden) {
		t.Fatalf("error.code = %q, want forbidden", body.Error.Code)
	}
}

func TestCreateReportValidationError(t *testing.T) {
	fake := newFakeReportService()
	server := NewServer(Config{ReportService: fake})

	req := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(`{}`))
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListReportsWithoutReportServiceReturnsDependencyError(t *testing.T) {
	server := NewServer(Config{})

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	req.Header.Set("X-User-Id", "user-1")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502, body = %s", rec.Code, rec.Body.String())
	}
}
