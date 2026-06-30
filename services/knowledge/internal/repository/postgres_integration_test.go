package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestPostgresRepositoryDocumentUploadLifecycle(t *testing.T) {
	repo, pool, cleanup := newPostgresRepositoryForTest(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC)
	scope := service.AccessScope{UserID: "usr_1", CanWrite: true}

	kb, err := repo.CreateKnowledgeBase(ctx, service.CreateKnowledgeBaseRecord{
		ID:                "kb_1",
		Name:              "规程库",
		Description:       "",
		DocType:           "GENERAL",
		ChunkStrategy:     json.RawMessage(`{"type":"fixed"}`),
		RetrievalStrategy: json.RawMessage(`{"mode":"vector"}`),
		CreatedBy:         "usr_1",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if kb.ID != "kb_1" || kb.CreatedBy != "usr_1" {
		t.Fatalf("knowledge base = %+v", kb)
	}

	doc, job, err := repo.CreateDocumentWithJob(ctx, service.CreateDocumentWithJobRecord{
		DocumentID:      "doc_1",
		KnowledgeBaseID: kb.ID,
		FileRef:         "file_1",
		Name:            "规程.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       9,
		Status:          service.DocumentStatusUploaded,
		Tags:            []string{"锅炉"},
		CurrentJobID:    "job_1",
		CreatedBy:       "usr_1",
		JobID:           "job_1",
		JobType:         service.JobTypeDocumentIngestion,
		JobStatus:       service.JobStatusQueued,
		JobStage:        "uploaded",
		JobMessage:      "document uploaded and queued for ingestion",
		MaxAttempts:     3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, scope)
	if err != nil {
		t.Fatalf("CreateDocumentWithJob() error = %v", err)
	}
	if doc.CurrentJobID == nil || *doc.CurrentJobID != job.ID {
		t.Fatalf("document/job link = %+v / %+v", doc, job)
	}
	if job.DocumentID == nil || *job.DocumentID != doc.ID || job.Status != service.JobStatusQueued {
		t.Fatalf("job = %+v", job)
	}

	list, err := repo.ListDocumentsByKnowledgeBase(ctx, kb.ID, nil, scope, service.PageInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListDocumentsByKnowledgeBase() error = %v", err)
	}
	if list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != doc.ID {
		t.Fatalf("document list = %+v", list)
	}

	failedAt := now.Add(time.Minute)
	if err := repo.MarkDocumentJobFailed(ctx, doc.ID, job.ID, "dependency_error", "queue failed", failedAt); err != nil {
		t.Fatalf("MarkDocumentJobFailed() error = %v", err)
	}
	failedDoc, err := repo.GetDocument(ctx, doc.ID, scope)
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if failedDoc.Status != service.DocumentStatusFailed ||
		failedDoc.ErrorCode == nil || *failedDoc.ErrorCode != "dependency_error" ||
		failedDoc.ErrorMessage == nil || *failedDoc.ErrorMessage != "queue failed" {
		t.Fatalf("failed document = %+v", failedDoc)
	}

	var jobStatus, jobErrorCode, jobErrorMessage string
	var jobFinishedAt, jobUpdatedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), COALESCE(error_message, ''), finished_at, updated_at
		FROM processing_jobs
		WHERE id = $1
	`, job.ID).Scan(&jobStatus, &jobErrorCode, &jobErrorMessage, &jobFinishedAt, &jobUpdatedAt); err != nil {
		t.Fatalf("query failed processing job: %v", err)
	}
	if jobStatus != string(service.JobStatusFailed) ||
		jobErrorCode != "dependency_error" ||
		jobErrorMessage != "queue failed" ||
		!jobFinishedAt.Equal(failedAt) ||
		!jobUpdatedAt.Equal(failedAt) {
		t.Fatalf("failed job status = %q errorCode = %q errorMessage = %q finishedAt = %s updatedAt = %s",
			jobStatus, jobErrorCode, jobErrorMessage, jobFinishedAt, jobUpdatedAt)
	}
}

func newPostgresRepositoryForTest(t *testing.T) (*repository.PostgresRepository, *pgxpool.Pool, func()) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("KNOWLEDGE_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run Postgres repository integration tests")
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	schema := fmt.Sprintf("knowledge_test_%d", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("parse test database url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("connect isolated test schema: %v", err)
	}

	applyKnowledgeMigration(t, ctx, pool)
	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
	}
	return repository.NewPostgresRepository(pool), pool, cleanup
}

func applyKnowledgeMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	for _, migration := range []string{
		"../../migrations/0001_create_knowledge_core_tables.sql",
		"../../migrations/0002_create_parser_configs.sql",
	} {
		contents, err := os.ReadFile(migration)
		if err != nil {
			t.Fatalf("read knowledge migration %s: %v", migration, err)
		}
		upSQL, _, _ := strings.Cut(string(contents), "-- +goose Down")
		upSQL = strings.ReplaceAll(upSQL, "-- +goose Up", "")

		for _, statement := range strings.Split(upSQL, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := pool.Exec(ctx, statement); err != nil {
				t.Fatalf("apply migration %s statement %q: %v", migration, statement, err)
			}
		}
	}
}
