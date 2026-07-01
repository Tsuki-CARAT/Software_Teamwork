package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepositoryDBSmoke(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("AI_GATEWAY_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set AI_GATEWAY_TEST_DATABASE_URL to run AI Gateway PostgreSQL DB smoke")
	}

	ctx := context.Background()
	pool := newSmokePool(t, ctx, databaseURL)
	repo := NewPostgresRepository(pool)

	key := sha256.Sum256([]byte("s-036-ai-gateway-db-smoke-encryption-key"))
	encryptor, err := service.NewCredentialEncryptor(key[:], "s036-smoke-key-v1")
	if err != nil {
		t.Fatalf("NewCredentialEncryptor() error = %v", err)
	}
	profiles := service.New(repo, encryptor, service.DefaultTimeoutMS)
	reqCtx := service.RequestContext{
		RequestID:     "req-s036-db-smoke",
		CallerService: "gateway",
		UserID:        "usr_s036",
	}

	dimensions := 1024
	enabled := true
	isDefault := true
	apiKey := "sk-s036-create-secret"
	created, err := profiles.CreateModelProfile(ctx, reqCtx, service.CreateModelProfileInput{
		ID:         "mp_s036_db_smoke",
		Name:       "s036-db-smoke-embedding",
		Purpose:    service.PurposeEmbedding,
		Provider:   service.ProviderOpenAICompatible,
		BaseURL:    "https://provider.example.test/v1",
		Model:      "embedding-smoke-model",
		APIKey:     apiKey,
		Enabled:    &enabled,
		IsDefault:  &isDefault,
		Dimensions: &dimensions,
	})
	if err != nil {
		t.Fatalf("CreateModelProfile() error = %v", err)
	}
	if created.ID != "mp_s036_db_smoke" || !created.APIKeyConfigured || created.CredentialID == "" {
		t.Fatalf("created profile = %+v", created)
	}

	credential, err := repo.GetActiveCredential(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetActiveCredential() error = %v", err)
	}
	if bytes.Contains(credential.Ciphertext, []byte(apiKey)) {
		t.Fatalf("credential ciphertext contains raw api key")
	}
	decrypted, err := encryptor.Decrypt(credential)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != apiKey {
		t.Fatalf("decrypted credential mismatch")
	}

	rotatedKey := "sk-s036-rotated-secret"
	updatedName := "s036-db-smoke-embedding-rotated"
	updated, err := profiles.UpdateModelProfile(ctx, reqCtx, service.UpdateModelProfileInput{
		ID:     created.ID,
		Name:   &updatedName,
		APIKey: &rotatedKey,
	})
	if err != nil {
		t.Fatalf("UpdateModelProfile() error = %v", err)
	}
	if updated.Name != updatedName || updated.CredentialID == "" || updated.CredentialID == credential.ID {
		t.Fatalf("updated profile = %+v, old credential = %s", updated, credential.ID)
	}
	assertCredentialStatus(t, ctx, pool, credential.ID, string(service.CredentialRotated))
	newCredential, err := repo.GetActiveCredential(ctx, updated.ID)
	if err != nil {
		t.Fatalf("GetActiveCredential() after rotation error = %v", err)
	}
	if newCredential.ID != updated.CredentialID || newCredential.Status != service.CredentialActive {
		t.Fatalf("active credential = %+v, want %s active", newCredential, updated.CredentialID)
	}
	if bytes.Contains(newCredential.Ciphertext, []byte(rotatedKey)) {
		t.Fatalf("rotated credential ciphertext contains raw api key")
	}

	statusCode := 200
	promptTokens := 5
	totalTokens := 5
	inputCount := 2
	startedAt := time.Date(2026, 7, 1, 3, 5, 0, 0, time.UTC)
	err = repo.RecordProviderInvocation(ctx, service.ProviderInvocation{
		ID:                  "pinv_s036_db_smoke",
		RequestID:           "req-s036-db-smoke-embedding",
		CallerService:       "knowledge",
		ExternalUserID:      "usr_s036",
		Operation:           service.OperationEmbedding,
		ProfileID:           updated.ID,
		Provider:            updated.Provider,
		Model:               updated.Model,
		Status:              service.InvocationSucceeded,
		ProviderStatusCode:  &statusCode,
		PromptTokens:        &promptTokens,
		TotalTokens:         &totalTokens,
		InputCount:          &inputCount,
		EmbeddingDimensions: &dimensions,
		DurationMS:          37,
		AttemptCount:        1,
		CreatedAt:           startedAt,
		FinishedAt:          startedAt.Add(37 * time.Millisecond),
	}, []service.ProviderInvocationAttempt{{
		ID:                 "pattempt_s036_db_smoke",
		InvocationID:       "pinv_s036_db_smoke",
		AttemptNo:          1,
		Provider:           updated.Provider,
		BaseURLHost:        "provider.example.test",
		Model:              updated.Model,
		Status:             service.InvocationSucceeded,
		ProviderStatusCode: &statusCode,
		DurationMS:         37,
		StartedAt:          startedAt,
		FinishedAt:         startedAt.Add(37 * time.Millisecond),
	}})
	if err != nil {
		t.Fatalf("RecordProviderInvocation() error = %v", err)
	}
	assertInvocationPersisted(t, ctx, pool, updated.ID)
	assertUsageAggregatePersisted(t, ctx, pool, updated.ID)

	if err := profiles.DeleteModelProfile(ctx, reqCtx, updated.ID); err != nil {
		t.Fatalf("DeleteModelProfile() error = %v", err)
	}
	if _, err := repo.GetModelProfile(ctx, updated.ID); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("GetModelProfile() after delete error = %v, want ErrNotFound", err)
	}
	assertActiveCredentialCount(t, ctx, pool, updated.ID, 0)
}

func newSmokePool(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	schema := "ai_gateway_s036_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
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

	scopedURL := databaseURLWithSearchPath(databaseURL, schema)
	migrationDB, err := sql.Open("pgx", scopedURL)
	if err != nil {
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("open migration db: %v", err)
	}
	if err := migrationDB.PingContext(ctx); err != nil {
		_ = migrationDB.Close()
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("ping migration db: %v", err)
	}
	applySmokeMigrations(t, ctx, migrationDB)

	pool, err := pgxpool.New(ctx, scopedURL)
	if err != nil {
		_ = migrationDB.Close()
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("create pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = migrationDB.Close()
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("ping pgx pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_ = migrationDB.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
	})
	return pool
}

func applySmokeMigrations(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("find migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no AI Gateway migrations found")
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		upSQL, ok := gooseUpSQL(string(raw))
		if !ok || strings.TrimSpace(upSQL) == "" {
			t.Fatalf("migration %s has no goose Up SQL", file)
		}
		if _, err := db.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
}

func gooseUpSQL(raw string) (string, bool) {
	var builder strings.Builder
	inUp := false
	for _, line := range strings.SplitAfter(raw, "\n") {
		switch strings.TrimSpace(line) {
		case "-- +goose Up":
			inUp = true
			continue
		case "-- +goose Down":
			return builder.String(), inUp
		}
		if inUp {
			builder.WriteString(line)
		}
	}
	return builder.String(), inUp
}

func assertCredentialStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, credentialID, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx, `SELECT status FROM provider_credentials WHERE id = $1`, credentialID).Scan(&got); err != nil {
		t.Fatalf("query credential status: %v", err)
	}
	if got != want {
		t.Fatalf("credential %s status = %q, want %q", credentialID, got, want)
	}
}

func assertInvocationPersisted(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileID string) {
	t.Helper()
	var callerService, operation, status string
	var totalTokens, inputCount int
	if err := pool.QueryRow(ctx, `
		SELECT caller_service, operation, status, total_tokens, input_count
		FROM provider_invocations
		WHERE id = 'pinv_s036_db_smoke' AND profile_id = $1`, profileID).Scan(&callerService, &operation, &status, &totalTokens, &inputCount); err != nil {
		t.Fatalf("query provider invocation: %v", err)
	}
	if callerService != "knowledge" || operation != service.OperationEmbedding || status != string(service.InvocationSucceeded) || totalTokens != 5 || inputCount != 2 {
		t.Fatalf("provider invocation = caller:%s operation:%s status:%s tokens:%d input:%d", callerService, operation, status, totalTokens, inputCount)
	}
}

func assertUsageAggregatePersisted(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileID string) {
	t.Helper()
	var requestCount, successCount, failureCount, totalTokens, durationMS int
	if err := pool.QueryRow(ctx, `
		SELECT request_count, success_count, failure_count, total_tokens, total_duration_ms
		FROM model_usage_aggregates
		WHERE caller_service = 'knowledge' AND profile_id = $1 AND operation = $2`, profileID, service.OperationEmbedding).Scan(&requestCount, &successCount, &failureCount, &totalTokens, &durationMS); err != nil {
		t.Fatalf("query usage aggregate: %v", err)
	}
	if requestCount != 1 || successCount != 1 || failureCount != 0 || totalTokens != 5 || durationMS != 37 {
		t.Fatalf("usage aggregate = requests:%d success:%d failure:%d tokens:%d duration:%d", requestCount, successCount, failureCount, totalTokens, durationMS)
	}
}

func assertActiveCredentialCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileID string, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provider_credentials WHERE profile_id = $1 AND status = 'active'`, profileID).Scan(&got); err != nil {
		t.Fatalf("query active credential count: %v", err)
	}
	if got != want {
		t.Fatalf("active credential count = %d, want %d", got, want)
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
