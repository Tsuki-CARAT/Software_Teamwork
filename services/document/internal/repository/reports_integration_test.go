package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
)

func TestPostgresRepositoryReportOutlineSectionLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DOCUMENT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DOCUMENT_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool := newTestPool(t, ctx, databaseURL)
	defer pool.Close()
	applyMigration(t, ctx, pool)

	repo := NewPostgresRepository(pool)
	now := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)

	reportType, err := repo.UpsertReportType(ctx, service.ReportType{
		Code:      "lifecycle_report",
		Name:      "Lifecycle Report",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("UpsertReportType() error = %v", err)
	}

	report, err := repo.CreateReport(ctx, service.Report{
		ID:         "00000000-0000-0000-0000-000000000901",
		Name:       "lifecycle report",
		ReportType: reportType.Code,
		Topic:      "lifecycle",
		Status:     service.ReportStatusDraft,
		CreatorID:  "user-1",
		Source:     "backend",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}

	fetched, err := repo.GetReportByID(ctx, report.ID)
	if err != nil {
		t.Fatalf("GetReportByID() error = %v", err)
	}
	if fetched.Name != "lifecycle report" {
		t.Fatalf("fetched.Name = %q", fetched.Name)
	}

	reports, total, err := repo.ListReports(ctx, service.ReportListFilter{CreatorID: "user-1"})
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if total != 1 || len(reports) != 1 {
		t.Fatalf("ListReports() total = %d, len = %d, want 1/1", total, len(reports))
	}

	updatedTopic := "lifecycle updated"
	fetched.Topic = updatedTopic
	fetched.UpdatedAt = now.Add(time.Minute)
	updated, err := repo.UpdateReport(ctx, fetched)
	if err != nil {
		t.Fatalf("UpdateReport() error = %v", err)
	}
	if updated.Topic != updatedTopic {
		t.Fatalf("updated.Topic = %q, want %q", updated.Topic, updatedTopic)
	}

	outline, err := repo.CreateReportOutline(ctx, service.ReportOutline{
		ID:       "00000000-0000-0000-0000-000000000902",
		ReportID: report.ID,
		Sections: []service.ReportOutlineNode{
			{ID: "node-1", Title: "Intro", Level: 1, Numbering: "1"},
			{ID: "node-2", Title: "Body", Level: 1, Numbering: "2", Children: []service.ReportOutlineNode{
				{ID: "node-2-1", Title: "Detail", Level: 2, Numbering: "2.1"},
			}},
		},
		Version:      1,
		Source:       service.OutlineSourceManual,
		IsCurrent:    true,
		ManualEdited: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("CreateReportOutline() error = %v", err)
	}
	if len(outline.Sections) != 2 || outline.Sections[1].Children[0].Title != "Detail" {
		t.Fatalf("unexpected round-tripped outline sections: %+v", outline.Sections)
	}

	outlines, err := repo.ListReportOutlines(ctx, report.ID)
	if err != nil {
		t.Fatalf("ListReportOutlines() error = %v", err)
	}
	if len(outlines) != 1 {
		t.Fatalf("ListReportOutlines() len = %d, want 1", len(outlines))
	}

	outline.Sections = outline.Sections[:1]
	outline.ManualEdited = true
	outline.UpdatedAt = now.Add(time.Minute)
	updatedOutline, err := repo.UpdateReportOutline(ctx, outline)
	if err != nil {
		t.Fatalf("UpdateReportOutline() error = %v", err)
	}
	if len(updatedOutline.Sections) != 1 {
		t.Fatalf("updatedOutline.Sections len = %d, want 1", len(updatedOutline.Sections))
	}

	section, err := repo.CreateReportSection(ctx, service.ReportSection{
		ID:               "00000000-0000-0000-0000-000000000903",
		ReportID:         report.ID,
		OutlineNodeID:    "node-1",
		SectionPath:      "00000000-0000-0000-0000-000000000903",
		Title:            "Intro",
		Level:            1,
		SortOrder:        0,
		SectionType:      service.SectionTypeText,
		Content:          "hello",
		Tables:           []map[string]any{{"rows": float64(1)}},
		GenerationStatus: service.JobStatusPending,
		ContentSource:    service.ContentSourceManual,
		ManualEdited:     true,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("CreateReportSection() error = %v", err)
	}
	if len(section.Tables) != 1 {
		t.Fatalf("section.Tables = %+v, want 1 entry", section.Tables)
	}

	sections, err := repo.ListReportSections(ctx, report.ID)
	if err != nil {
		t.Fatalf("ListReportSections() error = %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("ListReportSections() len = %d, want 1", len(sections))
	}

	section.Content = "updated content"
	section.Version = 2
	section.UpdatedAt = now.Add(time.Minute)
	updatedSection, err := repo.UpdateReportSection(ctx, section)
	if err != nil {
		t.Fatalf("UpdateReportSection() error = %v", err)
	}
	if updatedSection.Content != "updated content" || updatedSection.Version != 2 {
		t.Fatalf("unexpected updated section: %+v", updatedSection)
	}

	version, err := repo.CreateReportSectionVersion(ctx, service.ReportSectionVersion{
		ID:        "00000000-0000-0000-0000-000000000904",
		ReportID:  report.ID,
		SectionID: section.ID,
		Version:   1,
		Source:    service.ContentSourceManual,
		Content:   "v1 snapshot",
		CreatedBy: "user-1",
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateReportSectionVersion() error = %v", err)
	}
	if version.Content != "v1 snapshot" {
		t.Fatalf("version.Content = %q", version.Content)
	}

	versions, err := repo.ListReportSectionVersions(ctx, section.ID)
	if err != nil {
		t.Fatalf("ListReportSectionVersions() error = %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("ListReportSectionVersions() len = %d, want 1", len(versions))
	}

	deleted, err := repo.SoftDeleteReport(ctx, report.ID, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("SoftDeleteReport() error = %v", err)
	}
	if deleted.Status != service.ReportStatusDeleted || deleted.DeletedAt == nil {
		t.Fatalf("deleted report status = %q, deletedAt = %v", deleted.Status, deleted.DeletedAt)
	}

	listAfterDelete, _, err := repo.ListReports(ctx, service.ReportListFilter{CreatorID: "user-1"})
	if err != nil {
		t.Fatalf("ListReports() after delete error = %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Fatalf("expected deleted report to be excluded from default listing, got %d", len(listAfterDelete))
	}
}
