package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryPersistsReportJobAttemptAndEvent(t *testing.T) {
	databaseURL := os.Getenv("DOCUMENT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DOCUMENT_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool := newTestPool(t, ctx, databaseURL)
	defer pool.Close()
	applyMigration(t, ctx, pool)

	repo := NewPostgresRepository(pool)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)

	reportType, err := repo.UpsertReportType(ctx, service.ReportType{
		Code:      "integration_report",
		Name:      "Integration Report",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("UpsertReportType() error = %v", err)
	}
	if reportType.Code != "integration_report" {
		t.Fatalf("reportType.Code = %q", reportType.Code)
	}

	report, err := repo.CreateReport(ctx, service.Report{
		ID:         "00000000-0000-0000-0000-000000000101",
		Name:       "June baseline report",
		ReportType: reportType.Code,
		Topic:      "baseline",
		Status:     service.ReportStatusDraft,
		Source:     "backend",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}

	job, err := repo.CreateReportJob(ctx, service.ReportJob{
		ID:          "00000000-0000-0000-0000-000000000201",
		RequestID:   "req_integration",
		Source:      "api",
		JobType:     service.JobTypeOutlineGeneration,
		TargetType:  "report",
		TargetID:    report.ID,
		QueueName:   "document",
		ReportID:    report.ID,
		Status:      service.JobStatusPending,
		RetryCount:  0,
		MaxAttempts: 3,
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("CreateReportJob() error = %v", err)
	}
	if job.AsynqTaskID != "" {
		t.Fatalf("new job should not require asynq task id, got %q", job.AsynqTaskID)
	}
	if job.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts = %d, want 3", job.MaxAttempts)
	}

	attempt, err := repo.CreateReportJobAttempt(ctx, service.ReportJobAttempt{
		ID:            "00000000-0000-0000-0000-000000000301",
		JobID:         job.ID,
		AttemptNumber: 1,
		TriggerSource: "api",
		Status:        service.JobStatusPending,
		CreatedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreateReportJobAttempt() error = %v", err)
	}
	if attempt.AttemptNumber != 1 {
		t.Fatalf("AttemptNumber = %d, want 1", attempt.AttemptNumber)
	}

	event, err := repo.CreateReportEvent(ctx, service.ReportEvent{
		ID:        "00000000-0000-0000-0000-000000000401",
		ReportID:  report.ID,
		JobID:     job.ID,
		EventType: "job.created",
		Message:   "job created",
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateReportEvent() error = %v", err)
	}
	if event.EventType != "job.created" {
		t.Fatalf("EventType = %q", event.EventType)
	}

	foundJob, err := repo.FindReportJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("FindReportJobByID() error = %v", err)
	}
	if foundJob.Status != service.JobStatusPending {
		t.Fatalf("foundJob.Status = %q", foundJob.Status)
	}
}

func TestPostgresRepositoryWithinTxRollsBack(t *testing.T) {
	databaseURL := os.Getenv("DOCUMENT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DOCUMENT_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool := newTestPool(t, ctx, databaseURL)
	defer pool.Close()
	applyMigration(t, ctx, pool)

	repo := NewPostgresRepository(pool)
	now := time.Date(2026, 6, 29, 11, 0, 0, 0, time.UTC)
	reportType, err := repo.UpsertReportType(ctx, service.ReportType{
		Code:      "tx_report",
		Name:      "Transactional Report",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("UpsertReportType() error = %v", err)
	}
	report, err := repo.CreateReport(ctx, service.Report{
		ID:         "00000000-0000-0000-0000-000000000501",
		Name:       "rollback report",
		ReportType: reportType.Code,
		Topic:      "rollback",
		Status:     service.ReportStatusDraft,
		Source:     "backend",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}

	rollbackErr := errors.New("rollback requested")
	err = repo.WithinTx(ctx, func(txRepo *PostgresRepository) error {
		_, err := txRepo.CreateReportJob(ctx, service.ReportJob{
			ID:          "00000000-0000-0000-0000-000000000601",
			Source:      "api",
			JobType:     service.JobTypeOutlineGeneration,
			TargetType:  "report",
			TargetID:    report.ID,
			QueueName:   "document",
			ReportID:    report.ID,
			Status:      service.JobStatusPending,
			MaxAttempts: 3,
			CreatedAt:   now,
		})
		if err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("WithinTx() error = %v, want rollbackErr", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM report_jobs WHERE id = '00000000-0000-0000-0000-000000000601'`).Scan(&count); err != nil {
		t.Fatalf("count rolled back job: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back job count = %d, want 0", count)
	}
}

func newTestPool(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	schema := "document_test_" + strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", "_")
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect schema pool: %v", err)
	}
	return pool
}

func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	sqlBytes, err := os.ReadFile("../../migrations/0001_create_report_generation_tables.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	parts := strings.Split(string(sqlBytes), "-- +goose Down")
	up := strings.TrimPrefix(parts[0], "-- +goose Up")
	for _, stmt := range strings.Split(up, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("apply migration statement %q: %v", stmt, err)
		}
	}
}
