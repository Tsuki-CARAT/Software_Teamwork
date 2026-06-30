package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	filehttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/platform/storage"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/file/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const smokeServiceToken = "file-minio-postgres-smoke-token"

func TestFileMinIOPostgresSmoke(t *testing.T) {
	if os.Getenv("FILE_MINIO_POSTGRES_SMOKE") != "1" {
		t.Skip("set FILE_MINIO_POSTGRES_SMOKE=1 to run the real PostgreSQL + MinIO smoke")
	}

	cfg := loadSmokeConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := newIsolatedPostgresDB(t, ctx, cfg.databaseURL)
	applyMigrations(t, ctx, db)

	minioClient, err := storage.NewMinIOClient(storage.MinIOClientConfig{
		Endpoint:  cfg.minioEndpoint,
		AccessKey: cfg.minioAccessKey,
		SecretKey: cfg.minioSecretKey,
		UseSSL:    cfg.minioUseSSL,
		Region:    cfg.minioRegion,
		Timeout:   cfg.minioTimeout,
	})
	if err != nil {
		t.Fatalf("initialize MinIO client: %v", err)
	}
	store, err := storage.NewMinIOStore(minioClient, cfg.minioBucket)
	if err != nil {
		t.Fatalf("initialize MinIO store: %v", err)
	}

	files := service.New(
		repository.NewPostgresRepository(db),
		store,
		service.WithStorageBackend("minio"),
	)
	server := filehttp.NewServer(files, filehttp.Config{
		MaxUploadBytes:   1024 * 1024,
		ServiceToken:     smokeServiceToken,
		MetadataBackend:  "postgres",
		StorageBackend:   "minio",
		ReadinessChecker: postgresReadyChecker{db: db},
	})

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyReq.Header.Set("X-Request-Id", "req_file_smoke_ready")
	readyRes := httptest.NewRecorder()
	server.ServeHTTP(readyRes, readyReq)
	if readyRes.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, body = %s", readyRes.Code, readyRes.Body.String())
	}

	content := "minio postgres smoke content"
	checksum := sha256Hex(content)
	createReq := newMultipartUploadRequest(t, "/internal/v1/files", "smoke-policy.txt", "text/plain", content, checksum)
	createRes := httptest.NewRecorder()
	server.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRes.Code, createRes.Body.String())
	}
	assertNoSensitiveStorageDetails(t, createRes.Body.String(), cfg)

	var created fileResponseBody
	decodeJSON(t, createRes.Body, &created)
	if created.Data.ID == "" || !strings.HasPrefix(created.Data.ID, "file_") {
		t.Fatalf("created file id = %q", created.Data.ID)
	}
	if created.Data.Filename != "smoke-policy.txt" || created.Data.ContentType != "text/plain" {
		t.Fatalf("created metadata = %+v", created.Data)
	}
	if created.Data.SizeBytes != int64(len(content)) || created.Data.ChecksumSHA256 == nil || *created.Data.ChecksumSHA256 != checksum {
		t.Fatalf("created file content metadata = %+v, checksum = %q", created.Data, checksum)
	}

	row := loadFileObjectRow(t, ctx, db, created.Data.ID)
	if row.StorageBackend != "minio" || row.Status != "available" || row.ChecksumSHA256 != checksum || row.CreatedByService != "knowledge" {
		t.Fatalf("postgres row after create = %+v", row)
	}
	objectKey := loadStorageObjectKey(t, ctx, db, created.Data.ID)

	getReq := newInternalRequest(http.MethodGet, "/internal/v1/files/"+created.Data.ID, nil)
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRes.Code, getRes.Body.String())
	}
	assertNoSensitiveStorageDetails(t, getRes.Body.String(), cfg)

	contentReq := newInternalRequest(http.MethodGet, "/internal/v1/files/"+created.Data.ID+"/content", nil)
	contentRes := httptest.NewRecorder()
	server.ServeHTTP(contentRes, contentReq)
	if contentRes.Code != http.StatusOK {
		t.Fatalf("content status = %d, body = %s", contentRes.Code, contentRes.Body.String())
	}
	if got := contentRes.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("content type = %q", got)
	}
	if got := contentRes.Body.String(); got != content {
		t.Fatalf("content body = %q", got)
	}

	deleteReq := newInternalRequest(http.MethodDelete, "/internal/v1/files/"+created.Data.ID, nil)
	deleteRes := httptest.NewRecorder()
	server.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteRes.Code, deleteRes.Body.String())
	}

	deletedRow := loadFileObjectRow(t, ctx, db, created.Data.ID)
	if deletedRow.Status != "purged" || !deletedRow.DeletedAt.Valid || !deletedRow.PurgedAt.Valid {
		t.Fatalf("postgres row after delete = %+v", deletedRow)
	}
	storedAfterDelete, err := store.Get(ctx, objectKey)
	if err == nil {
		_ = storedAfterDelete.Body.Close()
		t.Fatal("MinIO object was still readable after File API delete")
	}
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("checking MinIO object after delete: %v", err)
	}

	getAfterDelete := newInternalRequest(http.MethodGet, "/internal/v1/files/"+created.Data.ID, nil)
	getAfterDeleteRes := httptest.NewRecorder()
	server.ServeHTTP(getAfterDeleteRes, getAfterDelete)
	if getAfterDeleteRes.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, body = %s", getAfterDeleteRes.Code, getAfterDeleteRes.Body.String())
	}

	contentAfterDelete := newInternalRequest(http.MethodGet, "/internal/v1/files/"+created.Data.ID+"/content", nil)
	contentAfterDeleteRes := httptest.NewRecorder()
	server.ServeHTTP(contentAfterDeleteRes, contentAfterDelete)
	if contentAfterDeleteRes.Code != http.StatusNotFound {
		t.Fatalf("content after delete status = %d, body = %s", contentAfterDeleteRes.Code, contentAfterDeleteRes.Body.String())
	}
}

type smokeConfig struct {
	databaseURL     string
	minioEndpoint   string
	minioAccessKey  string
	minioSecretKey  string
	minioBucket     string
	minioUseSSL     bool
	minioRegion     string
	minioTimeout    time.Duration
	sensitiveValues []string
}

func loadSmokeConfig(t *testing.T) smokeConfig {
	t.Helper()
	required := map[string]string{
		"FILE_TEST_DATABASE_URL": os.Getenv("FILE_TEST_DATABASE_URL"),
		"FILE_MINIO_ENDPOINT":    os.Getenv("FILE_MINIO_ENDPOINT"),
		"FILE_MINIO_ACCESS_KEY":  os.Getenv("FILE_MINIO_ACCESS_KEY"),
		"FILE_MINIO_SECRET_KEY":  os.Getenv("FILE_MINIO_SECRET_KEY"),
		"FILE_MINIO_BUCKET":      os.Getenv("FILE_MINIO_BUCKET"),
	}
	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("FILE_MINIO_POSTGRES_SMOKE=1 requires %s", strings.Join(missing, ", "))
	}

	useSSL := false
	if raw := strings.TrimSpace(os.Getenv("FILE_MINIO_USE_SSL")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			t.Fatalf("FILE_MINIO_USE_SSL must be a boolean: %v", err)
		}
		useSSL = value
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("FILE_MINIO_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			t.Fatalf("FILE_MINIO_TIMEOUT must be a positive duration")
		}
		timeout = value
	}

	cfg := smokeConfig{
		databaseURL:    strings.TrimSpace(required["FILE_TEST_DATABASE_URL"]),
		minioEndpoint:  strings.TrimSpace(required["FILE_MINIO_ENDPOINT"]),
		minioAccessKey: strings.TrimSpace(required["FILE_MINIO_ACCESS_KEY"]),
		minioSecretKey: strings.TrimSpace(required["FILE_MINIO_SECRET_KEY"]),
		minioBucket:    strings.TrimSpace(required["FILE_MINIO_BUCKET"]),
		minioUseSSL:    useSSL,
		minioRegion:    strings.TrimSpace(os.Getenv("FILE_MINIO_REGION")),
		minioTimeout:   timeout,
	}
	cfg.sensitiveValues = []string{
		cfg.databaseURL,
		cfg.minioEndpoint,
		cfg.minioAccessKey,
		cfg.minioSecretKey,
		cfg.minioBucket,
		"files/",
		"storageObjectKey",
		"storageBucket",
		"objectKey",
	}
	return cfg
}

func newIsolatedPostgresDB(t *testing.T, ctx context.Context, databaseURL string) *sql.DB {
	t.Helper()
	schema := "file_smoke_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")

	admin, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+quoteIdent(schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create smoke schema: %v", err)
	}

	db, err := sql.Open("pgx", databaseURLWithSearchPath(databaseURL, schema))
	if err != nil {
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("open smoke schema connection: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		_, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
		t.Fatalf("ping smoke schema connection: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`)
		_ = admin.Close()
	})
	return db
}

func applyMigrations(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	files, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("find file service migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no file service migrations found")
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

type fileObjectRow struct {
	StorageBackend   string
	Status           string
	ChecksumSHA256   string
	CreatedByService string
	DeletedAt        sql.NullTime
	PurgedAt         sql.NullTime
}

func loadFileObjectRow(t *testing.T, ctx context.Context, db *sql.DB, fileID string) fileObjectRow {
	t.Helper()
	const query = `
		SELECT storage_backend, status, checksum_sha256, created_by_service, deleted_at, purged_at
		FROM file_objects
		WHERE id = $1
	`
	var row fileObjectRow
	var checksum sql.NullString
	var createdByService sql.NullString
	if err := db.QueryRowContext(ctx, query, fileID).Scan(
		&row.StorageBackend,
		&row.Status,
		&checksum,
		&createdByService,
		&row.DeletedAt,
		&row.PurgedAt,
	); err != nil {
		t.Fatalf("load file_objects row %q: %v", fileID, err)
	}
	row.ChecksumSHA256 = checksum.String
	row.CreatedByService = createdByService.String
	return row
}

func loadStorageObjectKey(t *testing.T, ctx context.Context, db *sql.DB, fileID string) string {
	t.Helper()
	const query = `SELECT storage_object_key FROM file_objects WHERE id = $1`
	var objectKey string
	if err := db.QueryRowContext(ctx, query, fileID).Scan(&objectKey); err != nil {
		t.Fatalf("load storage object reference for %q: %v", fileID, err)
	}
	if strings.TrimSpace(objectKey) == "" {
		t.Fatalf("storage object reference for %q is empty", fileID)
	}
	return objectKey
}

func newMultipartUploadRequest(t *testing.T, target string, filename string, contentType string, content string, checksum string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if checksum != "" {
		if err := writer.WriteField("checksumSha256", checksum); err != nil {
			t.Fatalf("write checksum field: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := newInternalRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newInternalRequest(method string, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("X-Request-Id", "req_file_minio_pg_smoke")
	req.Header.Set("X-Caller-Service", "knowledge")
	req.Header.Set("X-Service-Token", smokeServiceToken)
	return req
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func assertNoSensitiveStorageDetails(t *testing.T, body string, cfg smokeConfig) {
	t.Helper()
	for _, value := range cfg.sensitiveValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(body, value) {
			t.Fatalf("response leaked storage detail %q: %s", value, body)
		}
	}
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
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

type postgresReadyChecker struct {
	db *sql.DB
}

func (c postgresReadyChecker) CheckReady(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

type fileResponseBody struct {
	Data struct {
		ID             string  `json:"id"`
		Filename       string  `json:"filename"`
		ContentType    string  `json:"contentType"`
		SizeBytes      int64   `json:"sizeBytes"`
		ChecksumSHA256 *string `json:"checksumSha256"`
		CreatedAt      string  `json:"createdAt"`
		DeletedAt      *string `json:"deletedAt"`
	} `json:"data"`
	RequestID string `json:"requestId"`
}
