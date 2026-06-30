package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepositoryFileObjectSmoke(t *testing.T) {
	databaseURL := os.Getenv("FILE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("FILE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db := newTestDB(t, ctx, databaseURL)
	applyMigration(t, ctx, db)

	repo := NewPostgresRepository(db)
	fileID := "file_integration_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	file := service.FileObject{
		ID:               fileID,
		Filename:         "policy.pdf",
		ContentType:      "application/pdf",
		SizeBytes:        7,
		ChecksumSHA256:   strings.Repeat("a", 64),
		StorageBackend:   "local",
		StorageObjectKey: "files/" + fileID,
		Status:           service.FileStatusAvailable,
		CreatedByService: "knowledge",
		RequestID:        "req_integration",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	created, err := repo.CreateFile(ctx, file)
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if created.ID != fileID || created.ChecksumSHA256 != file.ChecksumSHA256 || created.StorageObjectKey == "" {
		t.Fatalf("created file = %+v", created)
	}

	restartedRepo := NewPostgresRepository(db)
	found, err := restartedRepo.FindFileByID(ctx, fileID)
	if err != nil {
		t.Fatalf("FindFileByID() after repository restart error = %v", err)
	}
	if found.ID != fileID || found.Filename != "policy.pdf" || found.ChecksumSHA256 != file.ChecksumSHA256 {
		t.Fatalf("found file = %+v", found)
	}

	deleted, err := restartedRepo.MarkFileDeleteRequested(ctx, fileID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("MarkFileDeleteRequested() error = %v", err)
	}
	if deleted.Status != service.FileStatusDeleteRequested || deleted.DeletedAt == nil {
		t.Fatalf("deleted file = %+v", deleted)
	}
	if _, err := restartedRepo.FindFileByID(ctx, fileID); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("FindFileByID() after delete error = %v, want ErrNotFound", err)
	}

	purged, err := restartedRepo.MarkFilePurged(ctx, fileID, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("MarkFilePurged() error = %v", err)
	}
	if purged.Status != service.FileStatusPurged || purged.PurgedAt == nil {
		t.Fatalf("purged file = %+v", purged)
	}
}

func newTestDB(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
	t.Helper()
	schema := "file_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("ping admin db: %v", err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+quoteIdent(schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}

	db, err := sql.Open("pgx", databaseURLWithSearchPath(databaseURL, schema))
	if err != nil {
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("open schema db: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("ping schema db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
	})
	return db
}

func applyMigration(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("find migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no migrations found")
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		parts := strings.Split(string(sqlBytes), "-- +goose Down")
		up := strings.TrimPrefix(parts[0], "-- +goose Up")
		for _, stmt := range strings.Split(up, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("apply migration %s statement %q: %v", file, stmt, err)
			}
		}
	}
}

func databaseURLWithSearchPath(databaseURL string, schema string) string {
	parsed, err := url.Parse(databaseURL)
	if err == nil && parsed.Scheme != "" {
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return fmt.Sprintf("%s search_path=%s", databaseURL, schema)
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
