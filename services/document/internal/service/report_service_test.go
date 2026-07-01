package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeReportRepository is an in-memory ReportRepository used to unit test
// ReportService business rules without standing up PostgreSQL.
type fakeReportRepository struct {
	reports               map[string]Report
	outlines              map[string]ReportOutline
	sections              map[string]ReportSection
	sectionVersion        map[string][]ReportSectionVersion
	operationLogs         []OperationLog
	updateSectionErr      error
	beforeTx              func(*fakeReportRepository)
	reportForUpdateCalls  int
	sectionForUpdateCalls int
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

func (f *fakeReportRepository) GetReportByIDForUpdate(ctx context.Context, id string) (Report, error) {
	f.reportForUpdateCalls++
	return f.GetReportByID(ctx, id)
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

func (f *fakeReportRepository) GetReportSectionByIDForUpdate(ctx context.Context, id string) (ReportSection, error) {
	f.sectionForUpdateCalls++
	return f.GetReportSectionByID(ctx, id)
}

func (f *fakeReportRepository) UpdateReportSection(_ context.Context, value ReportSection) (ReportSection, error) {
	if f.updateSectionErr != nil {
		return ReportSection{}, f.updateSectionErr
	}
	if _, ok := f.sections[value.ID]; !ok {
		return ReportSection{}, NewError(CodeNotFound, "report section not found", nil)
	}
	f.sections[value.ID] = value
	return value, nil
}

func (f *fakeReportRepository) WithinTx(ctx context.Context, fn func(ReportRepository) error) error {
	if f.beforeTx != nil {
		beforeTx := f.beforeTx
		f.beforeTx = nil
		beforeTx(f)
	}
	snapshot := f.snapshot()
	if err := fn(f); err != nil {
		f.restore(snapshot)
		return err
	}
	return nil
}

func (f *fakeReportRepository) CreateReportSectionVersion(_ context.Context, value ReportSectionVersion) (ReportSectionVersion, error) {
	f.sectionVersion[value.SectionID] = append(f.sectionVersion[value.SectionID], value)
	return value, nil
}

func (f *fakeReportRepository) ListReportSectionVersions(_ context.Context, sectionID string) ([]ReportSectionVersion, error) {
	return f.sectionVersion[sectionID], nil
}

func (f *fakeReportRepository) CreateOperationLog(_ context.Context, log OperationLog) (OperationLog, error) {
	f.operationLogs = append(f.operationLogs, log)
	return log, nil
}

func (f *fakeReportRepository) snapshot() fakeReportRepository {
	snapshot := *f
	snapshot.reports = make(map[string]Report, len(f.reports))
	for id, report := range f.reports {
		snapshot.reports[id] = report
	}
	snapshot.outlines = make(map[string]ReportOutline, len(f.outlines))
	for id, outline := range f.outlines {
		snapshot.outlines[id] = outline
	}
	snapshot.sections = make(map[string]ReportSection, len(f.sections))
	for id, section := range f.sections {
		snapshot.sections[id] = section
	}
	snapshot.sectionVersion = make(map[string][]ReportSectionVersion, len(f.sectionVersion))
	for id, versions := range f.sectionVersion {
		snapshot.sectionVersion[id] = append([]ReportSectionVersion(nil), versions...)
	}
	snapshot.operationLogs = append([]OperationLog(nil), f.operationLogs...)
	return snapshot
}

func (f *fakeReportRepository) restore(snapshot fakeReportRepository) {
	reportForUpdateCalls := f.reportForUpdateCalls
	sectionForUpdateCalls := f.sectionForUpdateCalls
	*f = snapshot
	f.reportForUpdateCalls = reportForUpdateCalls
	f.sectionForUpdateCalls = sectionForUpdateCalls
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

func TestReportMutationsRecordOperationLogs(t *testing.T) {
	svc, repo := newTestService()
	reqCtx := RequestContext{UserID: "owner-1", RequestID: "req-report-audit"}

	report, err := svc.CreateReport(context.Background(), reqCtx, CreateReportInput{
		Name:       "June report",
		ReportType: "summer_peak_inspection",
		TemplateID: "tpl-1",
		Topic:      "summer peak",
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	if len(repo.operationLogs) != 1 {
		t.Fatalf("operation log count after create report = %d, want 1", len(repo.operationLogs))
	}
	if got := repo.operationLogs[0]; got.OperationType != OperationCreateReport || got.TargetID != report.ID {
		t.Fatalf("unexpected create report operation log: %+v", got)
	}

	topic := "updated"
	if _, err := svc.UpdateReport(context.Background(), reqCtx, report.ID, UpdateReportInput{Topic: &topic}); err != nil {
		t.Fatalf("UpdateReport() error = %v", err)
	}
	if got := repo.operationLogs[len(repo.operationLogs)-1]; got.OperationType != OperationUpdateReport || got.TargetType != "report" {
		t.Fatalf("unexpected update report operation log: %+v", got)
	}

	if err := svc.SoftDeleteReport(context.Background(), reqCtx, report.ID); err != nil {
		t.Fatalf("SoftDeleteReport() error = %v", err)
	}
	if got := repo.operationLogs[len(repo.operationLogs)-1]; got.OperationType != OperationDeleteReport || got.TargetID != report.ID {
		t.Fatalf("unexpected delete report operation log: %+v", got)
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

func TestUpdateSectionContentEditCannotBeUnmarkedAsManual(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	newContent := "edited body"
	manualEdited := false
	updated, err := svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{
		Content:      &newContent,
		ManualEdited: &manualEdited,
	})
	if err != nil {
		t.Fatalf("UpdateSection() error = %v", err)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected manualEdited to stay true even though the request set manualEdited:false alongside a content change")
	}
}

func TestSaveSectionsUpdatesExistingAndCreatesNewSections(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	existing, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
		Tables:  []map[string]any{{"name": "old"}},
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	newTitle := "Updated intro"
	newContent := "edited body"
	newTables := []map[string]any{{"name": "updated"}}
	createdTitle := "New section"
	createdContent := "new body"
	sections, err := svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{
				ID:      existing.ID,
				Title:   &newTitle,
				Content: &newContent,
				Tables:  &newTables,
			},
			{
				Title:   &createdTitle,
				Content: &createdContent,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("SaveSections() len = %d, want 2", len(sections))
	}

	updated := sections[0]
	if updated.ID != existing.ID {
		t.Fatalf("first section ID = %q, want %q", updated.ID, existing.ID)
	}
	if updated.Title != newTitle || updated.Content != newContent {
		t.Fatalf("updated section did not preserve requested fields: %+v", updated)
	}
	if updated.Version != existing.Version+1 {
		t.Fatalf("updated version = %d, want %d", updated.Version, existing.Version+1)
	}
	if !updated.ManualEdited {
		t.Fatalf("expected updated section to be marked manual edited")
	}

	created := sections[1]
	if created.ID == "" || created.ID == existing.ID {
		t.Fatalf("new section ID was not generated: %+v", created)
	}
	if created.Title != createdTitle || created.Content != createdContent {
		t.Fatalf("created section did not preserve requested fields: %+v", created)
	}
	if created.ManualEdited != true || created.Version != 1 {
		t.Fatalf("unexpected created manual/version fields: %+v", created)
	}
	if repo.sections[existing.ID].Content != newContent {
		t.Fatalf("repository did not persist updated section: %+v", repo.sections[existing.ID])
	}
	if _, ok := repo.sections[created.ID]; !ok {
		t.Fatalf("repository did not persist created section %q", created.ID)
	}
}

func TestSaveSectionsUpdatesMetadataWithoutBumpingVersion(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	existing, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:         "Intro",
		Level:         1,
		Numbering:     "1",
		OutlineNodeID: "outline-1",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	parent, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Parent"})
	if err != nil {
		t.Fatalf("CreateSection(parent) error = %v", err)
	}

	parentID := parent.ID
	outlineNodeID := "outline-2"
	title := "Updated intro"
	level := 2
	numbering := "1.1"
	manualEdited := false
	sections, err := svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{{
			ID:            existing.ID,
			ParentID:      &parentID,
			OutlineNodeID: &outlineNodeID,
			Title:         &title,
			Level:         &level,
			Numbering:     &numbering,
			ManualEdited:  &manualEdited,
		}},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("SaveSections() len = %d, want 1", len(sections))
	}

	updated := sections[0]
	if updated.ParentID != parentID || updated.OutlineNodeID != outlineNodeID || updated.Level != level || updated.Numbering != numbering {
		t.Fatalf("metadata fields were not saved: %+v", updated)
	}
	if updated.Title != title {
		t.Fatalf("Title = %q, want %q", updated.Title, title)
	}
	if updated.Version != existing.Version {
		t.Fatalf("metadata-only save bumped version to %d, want %d", updated.Version, existing.Version)
	}
	if updated.ManualEdited {
		t.Fatalf("metadata-only save should respect manualEdited=false when content is unchanged")
	}
}

func TestCreateSectionRejectsParentFromAnotherReport(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	otherReport := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	otherParent, err := svc.CreateSection(context.Background(), actor, otherReport.ID, CreateSectionInput{Title: "Other parent"})
	if err != nil {
		t.Fatalf("CreateSection(other parent) error = %v", err)
	}

	_, err = svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Child", ParentID: otherParent.ID})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Fields["parentId"] == "" {
		t.Fatalf("expected parentId validation error, got %v", err)
	}
}

func TestSaveSectionsRejectsParentCycle(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	first, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "First"})
	if err != nil {
		t.Fatalf("CreateSection(first) error = %v", err)
	}
	second, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Second"})
	if err != nil {
		t.Fatalf("CreateSection(second) error = %v", err)
	}

	firstParent := second.ID
	secondParent := first.ID
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{ID: first.ID, ParentID: &firstParent},
			{ID: second.ID, ParentID: &secondParent},
		},
	})
	appErr, ok := Classify(err)
	if !ok || appErr.Code != CodeValidation || appErr.Fields["parentId"] == "" {
		t.Fatalf("expected parentId cycle validation error, got %v", err)
	}
}

func TestSaveSectionsPersistsExplicitSortOrder(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	first, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "First"})
	if err != nil {
		t.Fatalf("CreateSection(first) error = %v", err)
	}
	second, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Second"})
	if err != nil {
		t.Fatalf("CreateSection(second) error = %v", err)
	}

	firstSortOrder := 1
	secondSortOrder := 0
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{
			{ID: second.ID, SortOrder: &secondSortOrder},
			{ID: first.ID, SortOrder: &firstSortOrder},
		},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if repo.sections[first.ID].SortOrder != firstSortOrder || repo.sections[second.ID].SortOrder != secondSortOrder {
		t.Fatalf("sortOrder was not persisted: first=%d second=%d", repo.sections[first.ID].SortOrder, repo.sections[second.ID].SortOrder)
	}
}

func TestSaveSectionsRechecksRunningStatusInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(f *fakeReportRepository) {
		current := f.sections[section.ID]
		current.GenerationStatus = JobStatusRunning
		current.LastJobID = "job-1"
		f.sections[section.ID] = current
	}

	newContent := "should not apply"
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{{
			ID:      section.ID,
			Content: &newContent,
		}},
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if repo.sectionForUpdateCalls != 1 {
		t.Fatalf("transactional section lock calls = %d, want 1", repo.sectionForUpdateCalls)
	}
	current := repo.sections[section.ID]
	if current.GenerationStatus != JobStatusRunning || current.LastJobID != "job-1" {
		t.Fatalf("running generation state was overwritten: %+v", current)
	}
	if current.Content != section.Content || current.Version != section.Version {
		t.Fatalf("section was modified despite transaction conflict: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("manual section versions were created despite transaction conflict: %+v", repo.sectionVersion[section.ID])
	}
}

func TestSaveSectionsRechecksDeletedReportInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(f *fakeReportRepository) {
		current := f.reports[report.ID]
		deletedAt := svc.now()
		current.Status = ReportStatusDeleted
		current.DeletedAt = &deletedAt
		f.reports[report.ID] = current
	}

	newContent := "should not apply"
	_, err = svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{{
			ID:      section.ID,
			Content: &newContent,
		}},
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if repo.reportForUpdateCalls != 1 {
		t.Fatalf("transactional report lock calls = %d, want 1", repo.reportForUpdateCalls)
	}
	current := repo.sections[section.ID]
	if current.Content != section.Content || current.Version != section.Version {
		t.Fatalf("section was modified after report deletion: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("manual section versions were created after report deletion: %+v", repo.sectionVersion[section.ID])
	}
}

func TestCreateSectionPersistsExplicitSortOrder(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	sortOrder := 5
	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:     "Sorted section",
		SortOrder: &sortOrder,
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.SortOrder != sortOrder {
		t.Fatalf("SortOrder = %d, want %d", section.SortOrder, sortOrder)
	}
}

func TestCreateSectionWithoutContentDefaultsToManualSource(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if section.ContentSource != ContentSourceManual {
		t.Fatalf("expected contentSource manual for a content-less section, got %q", section.ContentSource)
	}
	if section.ManualEdited {
		t.Fatalf("expected manualEdited = false for a section created without content")
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

func TestUpdateSectionRechecksRunningStatusInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(f *fakeReportRepository) {
		current := f.sections[section.ID]
		current.GenerationStatus = JobStatusRunning
		current.LastJobID = "job-1"
		f.sections[section.ID] = current
	}

	newContent := "should not apply"
	_, err = svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if repo.sectionForUpdateCalls != 1 {
		t.Fatalf("transactional section lock calls = %d, want 1", repo.sectionForUpdateCalls)
	}
	current := repo.sections[section.ID]
	if current.GenerationStatus != JobStatusRunning || current.LastJobID != "job-1" {
		t.Fatalf("running generation state was overwritten: %+v", current)
	}
	if current.Content != section.Content || current.Version != section.Version {
		t.Fatalf("section was modified despite transaction conflict: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("manual section versions were created despite transaction conflict: %+v", repo.sectionVersion[section.ID])
	}
}

func TestUpdateSectionRechecksDeletedReportInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{
		Title:   "Intro",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(f *fakeReportRepository) {
		current := f.reports[report.ID]
		deletedAt := svc.now()
		current.Status = ReportStatusDeleted
		current.DeletedAt = &deletedAt
		f.reports[report.ID] = current
	}

	newContent := "should not apply"
	_, err = svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &newContent})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if repo.reportForUpdateCalls != 1 {
		t.Fatalf("transactional report lock calls = %d, want 1", repo.reportForUpdateCalls)
	}
	current := repo.sections[section.ID]
	if current.Content != section.Content || current.Version != section.Version {
		t.Fatalf("section was modified after report deletion: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("manual section versions were created after report deletion: %+v", repo.sectionVersion[section.ID])
	}
}

func TestCreateSectionVersionSwitchesCurrentSectionInTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	content := "AI v2"
	tables := []map[string]any{{"name": "generated"}}
	version, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
		Tables:  &tables,
	})
	if err != nil {
		t.Fatalf("CreateSectionVersion() error = %v", err)
	}
	if version.Version != 2 || version.Content != content || version.Source != ContentSourceAI {
		t.Fatalf("unexpected created version: %+v", version)
	}

	current := repo.sections[section.ID]
	if current.Version != version.Version || current.Content != content {
		t.Fatalf("current section was not switched to created version: %+v", current)
	}
	if current.ContentSource != ContentSourceAI || current.ManualEdited || current.GenerationStatus != JobStatusSucceeded {
		t.Fatalf("current section source/manual/status not updated for AI version: %+v", current)
	}
	if current.GeneratedAt == nil {
		t.Fatalf("expected generatedAt to be set for AI version: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 1 {
		t.Fatalf("section version count = %d, want 1", len(repo.sectionVersion[section.ID]))
	}
}

func TestCreateSectionVersionConflictsWhileGenerationRunning(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	section.GenerationStatus = JobStatusRunning
	repo.sections[section.ID] = section

	content := "should not apply"
	_, err = svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("section versions were created despite conflict: %+v", repo.sectionVersion[section.ID])
	}
}

func TestCreateSectionVersionRejectsDeletedReport(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	if err := svc.SoftDeleteReport(context.Background(), actor, report.ID); err != nil {
		t.Fatalf("SoftDeleteReport() error = %v", err)
	}

	content := "should not apply"
	_, err = svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("section versions were created for deleted report: %+v", repo.sectionVersion[section.ID])
	}
	if got := repo.sections[section.ID]; got.Content != section.Content || got.Version != section.Version {
		t.Fatalf("section was modified for deleted report: %+v", got)
	}
}

func TestCreateSectionVersionRechecksDeletedReportInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(f *fakeReportRepository) {
		current := f.reports[report.ID]
		deletedAt := svc.now()
		current.Status = ReportStatusDeleted
		current.DeletedAt = &deletedAt
		f.reports[report.ID] = current
	}

	content := "should not apply"
	_, err = svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("section versions were created after report deletion: %+v", repo.sectionVersion[section.ID])
	}
	if got := repo.sections[section.ID]; got.Content != section.Content || got.Version != section.Version {
		t.Fatalf("section was modified after report deletion: %+v", got)
	}
	if repo.reportForUpdateCalls != 1 {
		t.Fatalf("transactional report lock calls = %d, want 1", repo.reportForUpdateCalls)
	}
}

func TestCreateSectionVersionRechecksRunningStatusInsideTransaction(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.beforeTx = func(repo *fakeReportRepository) {
		current := repo.sections[section.ID]
		current.GenerationStatus = JobStatusRunning
		repo.sections[section.ID] = current
	}

	content := "should not apply"
	_, err = svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
	})
	if code := errorCode(t, err); code != CodeConflict {
		t.Fatalf("error code = %q, want %q", code, CodeConflict)
	}
	current := repo.sections[section.ID]
	if current.GenerationStatus != JobStatusRunning {
		t.Fatalf("generation status = %q, want %q", current.GenerationStatus, JobStatusRunning)
	}
	if current.Content != section.Content || current.Version != section.Version {
		t.Fatalf("section was modified despite transaction conflict: %+v", current)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("section versions were created despite transaction conflict: %+v", repo.sectionVersion[section.ID])
	}
}

func TestCreateSectionVersionRollsBackWhenCurrentSectionUpdateFails(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}
	repo.updateSectionErr = errors.New("update current section failed")

	content := "AI v2"
	_, err = svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &content,
	})
	if code := errorCode(t, err); code != CodeDependency {
		t.Fatalf("error code = %q, want %q", code, CodeDependency)
	}
	if len(repo.sectionVersion[section.ID]) != 0 {
		t.Fatalf("created version was not rolled back: %+v", repo.sectionVersion[section.ID])
	}
	if got := repo.sections[section.ID]; got.Content != section.Content || got.Version != section.Version {
		t.Fatalf("section was modified despite rollback: %+v", got)
	}
}

func TestCreateAndListSectionVersionsKeepsManualAndAIHistory(t *testing.T) {
	svc, _ := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	manualContent := "manual v2"
	if _, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceManual,
		Content: &manualContent,
	}); err != nil {
		t.Fatalf("manual CreateSectionVersion() error = %v", err)
	}
	aiContent := "AI v3"
	if _, err := svc.CreateSectionVersion(context.Background(), actor, report.ID, section.ID, CreateSectionVersionInput{
		Source:  ContentSourceAI,
		Content: &aiContent,
	}); err != nil {
		t.Fatalf("AI CreateSectionVersion() error = %v", err)
	}

	versions, err := svc.ListSectionVersions(context.Background(), actor, report.ID, section.ID)
	if err != nil {
		t.Fatalf("ListSectionVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("version count = %d, want 2: %+v", len(versions), versions)
	}
	seen := map[ContentSource]string{}
	for _, version := range versions {
		seen[version.Source] = version.Content
	}
	if seen[ContentSourceManual] != manualContent || seen[ContentSourceAI] != aiContent {
		t.Fatalf("historical versions missing manual/AI content: %+v", versions)
	}
}

func TestUpdateSectionCreatesManualVersionSnapshot(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	content := "manual v2"
	updated, err := svc.UpdateSection(context.Background(), actor, report.ID, section.ID, UpdateSectionInput{Content: &content})
	if err != nil {
		t.Fatalf("UpdateSection() error = %v", err)
	}

	versions := repo.sectionVersion[section.ID]
	if len(versions) != 1 {
		t.Fatalf("manual version count = %d, want 1", len(versions))
	}
	if versions[0].Version != updated.Version || versions[0].Content != content || versions[0].Source != ContentSourceManual {
		t.Fatalf("unexpected manual snapshot: %+v", versions[0])
	}
}

func TestSaveSectionsCreatesManualVersionSnapshotForContentChanges(t *testing.T) {
	svc, repo := newTestService()
	report := mustCreateReport(t, svc, "owner-1")
	actor := RequestContext{UserID: "owner-1"}

	section, err := svc.CreateSection(context.Background(), actor, report.ID, CreateSectionInput{Title: "Intro", Content: "v1"})
	if err != nil {
		t.Fatalf("CreateSection() error = %v", err)
	}

	content := "manual v2"
	sections, err := svc.SaveSections(context.Background(), actor, report.ID, SaveSectionsInput{
		Sections: []SaveSectionInput{{ID: section.ID, Content: &content}},
	})
	if err != nil {
		t.Fatalf("SaveSections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("SaveSections() len = %d, want 1", len(sections))
	}

	versions := repo.sectionVersion[section.ID]
	if len(versions) != 1 {
		t.Fatalf("manual version count = %d, want 1", len(versions))
	}
	if versions[0].Version != sections[0].Version || versions[0].Content != content || versions[0].Source != ContentSourceManual {
		t.Fatalf("unexpected manual snapshot: %+v", versions[0])
	}
}
