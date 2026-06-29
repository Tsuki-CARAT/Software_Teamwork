package service

import (
	"context"
	"testing"
	"time"
)

// fakeReportRepository is an in-memory ReportRepository used to unit test
// ReportService business rules without standing up PostgreSQL.
type fakeReportRepository struct {
	reports        map[string]Report
	outlines       map[string]ReportOutline
	sections       map[string]ReportSection
	sectionVersion map[string][]ReportSectionVersion
}

func newFakeReportRepository() *fakeReportRepository {
	return &fakeReportRepository{
		reports:        map[string]Report{},
		outlines:       map[string]ReportOutline{},
		sections:       map[string]ReportSection{},
		sectionVersion: map[string][]ReportSectionVersion{},
	}
}

func (f *fakeReportRepository) CreateReport(_ context.Context, value Report) (Report, error) {
	f.reports[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) GetReportByID(_ context.Context, id string) (Report, error) {
	report, ok := f.reports[id]
	if !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	return report, nil
}

func (f *fakeReportRepository) ListReports(_ context.Context, filter ReportListFilter) ([]Report, int, error) {
	var result []Report
	for _, report := range f.reports {
		if filter.CreatorID != "" && report.CreatorID != filter.CreatorID {
			continue
		}
		result = append(result, report)
	}
	return result, len(result), nil
}

func (f *fakeReportRepository) UpdateReport(_ context.Context, value Report) (Report, error) {
	if _, ok := f.reports[value.ID]; !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	f.reports[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) SoftDeleteReport(_ context.Context, id string, deletedAt time.Time) (Report, error) {
	report, ok := f.reports[id]
	if !ok {
		return Report{}, NewError(CodeNotFound, "report not found", nil)
	}
	report.Status = ReportStatusDeleted
	report.DeletedAt = &deletedAt
	f.reports[id] = report
	return report, nil
}

func (f *fakeReportRepository) CreateReportOutline(_ context.Context, value ReportOutline) (ReportOutline, error) {
	if value.IsCurrent {
		for id, outline := range f.outlines {
			if outline.ReportID == value.ReportID {
				outline.IsCurrent = false
				f.outlines[id] = outline
			}
		}
	}
	f.outlines[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) ListReportOutlines(_ context.Context, reportID string) ([]ReportOutline, error) {
	var result []ReportOutline
	for _, outline := range f.outlines {
		if outline.ReportID == reportID {
			result = append(result, outline)
		}
	}
	return result, nil
}

func (f *fakeReportRepository) GetReportOutlineByID(_ context.Context, id string) (ReportOutline, error) {
	outline, ok := f.outlines[id]
	if !ok {
		return ReportOutline{}, NewError(CodeNotFound, "report outline not found", nil)
	}
	return outline, nil
}

func (f *fakeReportRepository) UpdateReportOutline(_ context.Context, value ReportOutline) (ReportOutline, error) {
	if _, ok := f.outlines[value.ID]; !ok {
		return ReportOutline{}, NewError(CodeNotFound, "report outline not found", nil)
	}
	f.outlines[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) CreateReportSection(_ context.Context, value ReportSection) (ReportSection, error) {
	f.sections[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) ListReportSections(_ context.Context, reportID string) ([]ReportSection, error) {
	var result []ReportSection
	for _, section := range f.sections {
		if section.ReportID == reportID {
			result = append(result, section)
		}
	}
	return result, nil
}

func (f *fakeReportRepository) GetReportSectionByID(_ context.Context, id string) (ReportSection, error) {
	section, ok := f.sections[id]
	if !ok {
		return ReportSection{}, NewError(CodeNotFound, "report section not found", nil)
	}
	return section, nil
}

func (f *fakeReportRepository) UpdateReportSection(_ context.Context, value ReportSection) (ReportSection, error) {
	if _, ok := f.sections[value.ID]; !ok {
		return ReportSection{}, NewError(CodeNotFound, "report section not found", nil)
	}
	f.sections[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) CreateReportSectionVersion(_ context.Context, value ReportSectionVersion) (ReportSectionVersion, error) {
	f.sectionVersion[value.SectionID] = append(f.sectionVersion[value.SectionID], value)
	return value, nil
}

func (f *fakeReportRepository) ListReportSectionVersions(_ context.Context, sectionID string) ([]ReportSectionVersion, error) {
	return f.sectionVersion[sectionID], nil
}

func newTestService() (*ReportService, *fakeReportRepository) {
	repo := newFakeReportRepository()
	svc := NewReportService(repo)
	svc.clock = func() time.Time { return time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC) }
	return svc, repo
}

func mustCreateReport(t *testing.T, svc *ReportService, owner string) Report {
	t.Helper()
	report, err := svc.CreateReport(context.Background(), RequestContext{UserID: owner}, CreateReportInput{
		Name:       "June report",
		ReportType: "summer_peak_inspection",
		TemplateID: "tpl-1",
		Topic:      "summer peak",
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	return report
}

func TestCreateReportValidatesRequiredFields(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.CreateReport(context.Background(), RequestContext{UserID: "u1"}, CreateReportInput{})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestStandardUserCannotAccessOthersReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")

	_, err := svc.GetReport(context.Background(), RequestContext{UserID: "intruder"}, report.ID)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeForbidden {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestAdminCanAccessOthersReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")

	got, err := svc.GetReport(context.Background(), RequestContext{UserID: "admin-1", Roles: []string{"admin"}}, report.ID)
	if err != nil {
		t.Fatalf("admin GetReport() error = %v", err)
	}
	if got.ID != report.ID {
		t.Fatalf("got report %q, want %q", got.ID, report.ID)
	}
}

func TestListReportsScopedToOwnerForStandardUser(t *testing.T) {
	svc, _ := newTestService()
	mustCreateReport(t, svc, "owner-1")
	mustCreateReport(t, svc, "owner-2")

	result, err := svc.ListReports(context.Background(), RequestContext{UserID: "owner-1"}, ReportListFilter{})
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if result.Page.Total != 1 || len(result.Items) != 1 || result.Items[0].CreatorID != "owner-1" {
		t.Fatalf("expected only owner-1's report, got %+v", result)
	}
}

func TestSoftDeleteReportIsIdempotentAndConflicts(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	if err := svc.SoftDeleteReport(context.Background(), actor, report.ID); err != nil {
		t.Fatalf("first SoftDeleteReport() error = %v", err)
	}

	err := svc.SoftDeleteReport(context.Background(), actor, report.ID)
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict on second delete, got %v", err)
	}
}

func TestUpdateReportRejectsDeletedReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}
	if err := svc.SoftDeleteReport(context.Background(), actor, report.ID); err != nil {
		t.Fatalf("SoftDeleteReport() error = %v", err)
	}

	newTopic := "updated topic"
	_, err := svc.UpdateReport(context.Background(), actor, report.ID, UpdateReportInput{Topic: &newTopic})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict updating deleted report, got %v", err)
	}
}

func TestCreateOutlineRenumbersAndVersions(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source: OutlineSourceManual,
		Sections: []ReportOutlineNode{
			{Title: "Intro"},
			{Title: "Body", Children: []ReportOutlineNode{{Title: "Detail"}}},
		},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}
	if outline.Version != 1 || !outline.IsCurrent {
		t.Fatalf("unexpected outline version/current: %+v", outline)
	}
	if outline.Sections[1].Children[0].Numbering != "2.1" {
		t.Fatalf("expected renumbered child 2.1, got %q", outline.Sections[1].Children[0].Numbering)
	}

	second, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source:   OutlineSourceAI,
		Sections: []ReportOutlineNode{{Title: "Regenerated"}},
	})
	if err != nil {
		t.Fatalf("second CreateOutline() error = %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
}

func TestDeleteOutlineSectionRenumbersRemaining(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source: OutlineSourceManual,
		Sections: []ReportOutlineNode{
			{Title: "Intro"},
			{Title: "Body"},
			{Title: "Conclusion"},
		},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}
	bodyID := outline.Sections[1].ID

	updated, err := svc.DeleteOutlineSection(context.Background(), actor, report.ID, outline.ID, bodyID)
	if err != nil {
		t.Fatalf("DeleteOutlineSection() error = %v", err)
	}
	if len(updated.Sections) != 2 {
		t.Fatalf("expected 2 remaining sections, got %d", len(updated.Sections))
	}
	if updated.Sections[1].Numbering != "2" {
		t.Fatalf("expected conclusion renumbered to 2, got %q", updated.Sections[1].Numbering)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited = true after delete")
	}
}

func TestDeleteOutlineSectionNotFound(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}
	outline, err := svc.CreateOutline(context.Background(), actor, report.ID, CreateOutlineInput{
		Source:   OutlineSourceManual,
		Sections: []ReportOutlineNode{{Title: "Intro"}},
	})
	if err != nil {
		t.Fatalf("CreateOutline() error = %v", err)
	}

	_, err = svc.DeleteOutlineSection(context.Background(), actor, report.ID, outline.ID, "missing-node")
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeNotFound {
		t.Fatalf("expected not_found error, got %v", err)
	}
}

func TestUpdateSectionMarksManualEditedAndBumpsVersion(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", section.Version)
	}

	newContent := "edited body"
	updated, err := svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	if err != nil {
		t.Fatalf("UpdateSection() error = %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version bumped to 2, got %d", updated.Version)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited = true")
	}
	if updated.ContentSource != ContentSourceManual {
		t.Fatalf("expected contentSource manual, got %q", updated.ContentSource)
	}
}

func TestUpdateSectionConflictsWhileGenerationRunning(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	section.GenerationStatus = JobStatusRunning
	repo.sections[section.ID] = section

	newContent := "should not apply"
	_, err = svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeConflict {
		t.Fatalf("expected conflict while generation running, got %v", err)
	}
}

func TestCreateSectionVersionDoesNotRequireRegeneration(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	version, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{Source: ContentSourceManual})
	if err != nil {
		t.Fatalf("CreateSectionVersion() error = %v", err)
	}
	if version.Version != 1 || version.Content != "v1" {
		t.Fatalf("unexpected first version: %+v", version)
	}

	second, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{Source: ContentSourceManual})
	if err != nil {
		t.Fatalf("CreateSectionVersion() error = %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
}
