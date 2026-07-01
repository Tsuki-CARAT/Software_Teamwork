package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/embedding"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/fileclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	vectorplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/worker"
)

const knowledgeIngestionSmokeGate = "KNOWLEDGE_INGESTION_SMOKE"

func TestKnowledgeIngestionRealDepsSmoke(t *testing.T) {
	if os.Getenv(knowledgeIngestionSmokeGate) != "1" {
		t.Skip("set KNOWLEDGE_INGESTION_SMOKE=1 to run the Knowledge ingestion real dependency smoke")
	}

	cfg := loadKnowledgeSmokeConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	runID := newSmokeRunID(t)
	collection := qdrantCollectionName(cfg.qdrantCollectionPrefix, runID)
	createQdrantCollection(t, ctx, cfg, collection)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := deleteQdrantCollection(cleanupCtx, cfg, collection); err != nil {
			t.Errorf("cleanup Qdrant collection %q: %v", collection, err)
		}
	})

	repo, cleanupRepo := newPostgresRepositoryForSmoke(t, ctx, cfg.databaseURL, runID)
	t.Cleanup(cleanupRepo)

	fileClient, err := fileclient.New(cfg.fileServiceBaseURL, cfg.serviceToken, nil)
	if err != nil {
		t.Fatalf("initialize File Service client: %v", err)
	}
	documentParser, err := parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL:      cfg.parserServiceBaseURL,
		ServiceToken: cfg.parserServiceToken,
		Timeout:      cfg.parserServiceTimeout,
	})
	if err != nil {
		t.Fatalf("initialize Parser Service client: %v", err)
	}
	embedder := newSmokeEmbedder(t, cfg)
	vectorIndex, err := vectorplatform.NewQdrantClient(vectorplatform.QdrantConfig{
		BaseURL:    cfg.qdrantURL,
		APIKey:     cfg.qdrantAPIKey,
		Collection: collection,
		Dimension:  cfg.embeddingDimension,
	})
	if err != nil {
		t.Fatalf("initialize Qdrant client: %v", err)
	}

	queue := &capturingIngestionQueue{}
	knowledge := service.NewWithDependencies(
		repo,
		fileClient,
		queue,
		nil,
		nil,
		service.WithProcessingPipeline(fileClient, documentParser, service.NewFixedChunker()),
		service.WithVectorIndex(embedder, vectorIndex, collection),
	)
	handler := worker.NewIngestionHandler(
		knowledge,
		worker.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	reqCtx := service.RequestContext{
		RequestID:     "req_knowledge_ingestion_smoke_" + runID,
		UserID:        "usr_knowledge_ingestion_smoke",
		CallerService: "knowledge-smoke",
		ServiceToken:  cfg.serviceToken,
		Roles:         []string{"admin"},
		Permissions:   []string{service.PermissionKnowledgeRead, service.PermissionKnowledgeWrite},
	}
	kb, err := knowledge.CreateKnowledgeBase(ctx, reqCtx, service.CreateKnowledgeBaseInput{
		ID:          "kb_smoke_" + runID,
		Name:        "Knowledge ingestion smoke " + runID,
		Description: stringPtr("A-021 real dependency smoke fixture"),
		DocType:     stringPtr("SMOKE"),
	})
	if err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	fixture := []byte("# Knowledge ingestion smoke fixture\n\n" +
		"Generator protection inspection requires confirmed relay settings, " +
		"documented handoff notes, and a ready retrieval index.\n")
	doc, err := knowledge.UploadDocument(ctx, reqCtx, service.UploadDocumentInput{
		KnowledgeBaseID: kb.ID,
		File: service.UploadedFile{
			Filename:    "knowledge-ingestion-smoke.md",
			ContentType: "text/markdown",
			SizeBytes:   int64(len(fixture)),
			Content:     bytes.NewReader(fixture),
		},
		Tags: []string{"smoke", "a021"},
	})
	if err != nil {
		t.Fatalf("upload fixture document: %v", err)
	}
	if doc.FileRef != nil {
		fileID := strings.TrimSpace(*doc.FileRef)
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cleanupCancel()
			if err := fileClient.DeleteFile(cleanupCtx, reqCtx, fileID); err != nil {
				t.Errorf("cleanup File Service object: %v", err)
			}
		})
	}

	task, ok := queue.singleTask()
	if !ok {
		t.Fatal("upload did not enqueue an ingestion task")
	}
	if task.RequestID != reqCtx.RequestID || task.JobID == "" || task.DocumentID != doc.ID || task.KnowledgeBaseID != kb.ID || task.UserID != reqCtx.UserID {
		t.Fatalf("queued ingestion task = %+v, want request/document/knowledge base/user ids", task)
	}
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal ingestion task: %v", err)
	}
	if err := handler.HandleIngestionPayload(ctx, payload); err != nil {
		t.Fatalf("run ingestion worker handler: %v", err)
	}

	readyDoc, err := knowledge.GetDocument(ctx, reqCtx, doc.ID)
	if err != nil {
		t.Fatalf("load ingested document: %v", err)
	}
	if readyDoc.Status != service.DocumentStatusReady {
		t.Fatalf("document status = %q, want ready", readyDoc.Status)
	}
	if readyDoc.ChunkCount <= 0 {
		t.Fatalf("document chunk count = %d, want > 0", readyDoc.ChunkCount)
	}
	if readyDoc.ParserBackend == nil || strings.TrimSpace(*readyDoc.ParserBackend) == "" {
		t.Fatal("document parser backend was not recorded")
	}
	if readyDoc.CurrentJobID == nil || strings.TrimSpace(*readyDoc.CurrentJobID) == "" {
		t.Fatal("document current job id was not recorded")
	}

	job, err := knowledge.GetJob(ctx, reqCtx, *readyDoc.CurrentJobID)
	if err != nil {
		t.Fatalf("load ingestion job: %v", err)
	}
	if job.Status != service.JobStatusSucceeded {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	chunkList, err := knowledge.ListChunks(ctx, reqCtx, service.ListChunksInput{
		DocumentID: doc.ID,
		Page:       service.PageInput{Page: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("list ingested chunks: %v", err)
	}
	if len(chunkList.Items) == 0 {
		t.Fatal("no chunks were persisted")
	}
	firstChunk := chunkList.Items[0]
	assertChunkIndexed(t, firstChunk, cfg)

	pointPayload := retrieveQdrantPointPayload(t, ctx, cfg, collection, strings.TrimSpace(*firstChunk.QdrantPointID))
	assertQdrantPayload(t, pointPayload, kb.ID, doc.ID, firstChunk.ID)
}

type knowledgeSmokeConfig struct {
	databaseURL            string
	fileServiceBaseURL     string
	serviceToken           string
	parserServiceBaseURL   string
	parserServiceToken     string
	parserServiceTimeout   time.Duration
	qdrantURL              string
	qdrantAPIKey           string
	qdrantCollectionPrefix string
	embeddingProvider      string
	embeddingModel         string
	embeddingDimension     int
	aiGatewayBaseURL       string
	aiGatewayServiceToken  string
	aiGatewayProfileID     string
}

func loadKnowledgeSmokeConfig(t *testing.T) knowledgeSmokeConfig {
	t.Helper()

	required := map[string]string{
		"FILE_SERVICE_BASE_URL":       os.Getenv("FILE_SERVICE_BASE_URL"),
		"KNOWLEDGE_SERVICE_TOKEN":     os.Getenv("KNOWLEDGE_SERVICE_TOKEN"),
		"KNOWLEDGE_TEST_DATABASE_URL": os.Getenv("KNOWLEDGE_TEST_DATABASE_URL"),
		"PARSER_SERVICE_BASE_URL":     os.Getenv("PARSER_SERVICE_BASE_URL"),
	}
	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	qdrantURL := firstNonEmptyEnv("QDRANT_URL", "QDRANT_BASE_URL")
	if strings.TrimSpace(qdrantURL) == "" {
		missing = append(missing, "QDRANT_URL or QDRANT_BASE_URL")
	}

	embeddingProvider := envOrDefault("EMBEDDING_PROVIDER", "local_hashing")
	embeddingModel := envOrDefault("EMBEDDING_MODEL", "local_hashing")
	embeddingDimension := positiveIntEnv(t, "EMBEDDING_DIMENSION", 384)
	aiGatewayBaseURL := strings.TrimSpace(os.Getenv("AI_GATEWAY_BASE_URL"))
	aiGatewayServiceToken := strings.TrimSpace(os.Getenv("AI_GATEWAY_SERVICE_TOKEN"))
	if strings.EqualFold(embeddingProvider, "ai_gateway") {
		if strings.TrimSpace(os.Getenv("EMBEDDING_MODEL")) == "" {
			missing = append(missing, "EMBEDDING_MODEL")
		}
		if strings.TrimSpace(os.Getenv("EMBEDDING_DIMENSION")) == "" {
			missing = append(missing, "EMBEDDING_DIMENSION")
		}
		if aiGatewayBaseURL == "" {
			missing = append(missing, "AI_GATEWAY_BASE_URL")
		}
		if aiGatewayServiceToken == "" {
			missing = append(missing, "AI_GATEWAY_SERVICE_TOKEN")
		}
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("KNOWLEDGE_INGESTION_SMOKE=1 requires %s", strings.Join(missing, ", "))
	}

	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("PARSER_SERVICE_TIMEOUT")); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			t.Fatalf("PARSER_SERVICE_TIMEOUT must be a positive duration")
		}
		timeout = value
	}

	return knowledgeSmokeConfig{
		databaseURL:            strings.TrimSpace(required["KNOWLEDGE_TEST_DATABASE_URL"]),
		fileServiceBaseURL:     strings.TrimRight(strings.TrimSpace(required["FILE_SERVICE_BASE_URL"]), "/"),
		serviceToken:           strings.TrimSpace(required["KNOWLEDGE_SERVICE_TOKEN"]),
		parserServiceBaseURL:   strings.TrimRight(strings.TrimSpace(required["PARSER_SERVICE_BASE_URL"]), "/"),
		parserServiceToken:     strings.TrimSpace(os.Getenv("PARSER_SERVICE_TOKEN")),
		parserServiceTimeout:   timeout,
		qdrantURL:              strings.TrimRight(strings.TrimSpace(qdrantURL), "/"),
		qdrantAPIKey:           strings.TrimSpace(os.Getenv("QDRANT_API_KEY")),
		qdrantCollectionPrefix: envOrDefault("KNOWLEDGE_SMOKE_QDRANT_COLLECTION_PREFIX", "knowledge_ingestion_smoke"),
		embeddingProvider:      embeddingProvider,
		embeddingModel:         embeddingModel,
		embeddingDimension:     embeddingDimension,
		aiGatewayBaseURL:       strings.TrimRight(aiGatewayBaseURL, "/"),
		aiGatewayServiceToken:  aiGatewayServiceToken,
		aiGatewayProfileID:     firstNonEmptyEnv("AI_GATEWAY_EMBEDDING_PROFILE_ID", "EMBEDDING_PROFILE_ID"),
	}
}

func newSmokeEmbedder(t *testing.T, cfg knowledgeSmokeConfig) service.Embedder {
	t.Helper()
	if strings.EqualFold(cfg.embeddingProvider, "ai_gateway") {
		client, err := embedding.NewAIGatewayClient(embedding.AIGatewayConfig{
			BaseURL:      cfg.aiGatewayBaseURL,
			Model:        cfg.embeddingModel,
			ProfileID:    cfg.aiGatewayProfileID,
			Dimensions:   cfg.embeddingDimension,
			ServiceToken: cfg.aiGatewayServiceToken,
		})
		if err != nil {
			t.Fatalf("initialize AI Gateway embedding client: %v", err)
		}
		return client
	}
	return embedding.NewLocalHasher(cfg.embeddingProvider, cfg.embeddingModel, cfg.embeddingDimension)
}

func newPostgresRepositoryForSmoke(t *testing.T, ctx context.Context, databaseURL string, runID string) (*repository.PostgresRepository, func()) {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect smoke PostgreSQL admin pool: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Fatalf("ping smoke PostgreSQL: %v", err)
	}

	schema := "knowledge_smoke_" + safeIdentifierSuffix(runID)
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		adminPool.Close()
		t.Fatalf("create smoke schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("parse smoke database URL: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("connect isolated smoke schema: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
		t.Fatalf("ping isolated smoke schema: %v", err)
	}

	applyKnowledgeMigrations(t, ctx, pool)
	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE")
		adminPool.Close()
	}
	return repository.NewPostgresRepository(pool), cleanup
}

func applyKnowledgeMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
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

type capturingIngestionQueue struct {
	mu    sync.Mutex
	tasks []service.DocumentIngestionTask
}

func (q *capturingIngestionQueue) EnqueueDocumentIngestion(ctx context.Context, task service.DocumentIngestionTask) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
	return nil
}

func (q *capturingIngestionQueue) singleTask() (service.DocumentIngestionTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tasks) != 1 {
		return service.DocumentIngestionTask{}, false
	}
	return q.tasks[0], true
}

func createQdrantCollection(t *testing.T, ctx context.Context, cfg knowledgeSmokeConfig, collection string) {
	t.Helper()
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     cfg.embeddingDimension,
			"distance": "Cosine",
		},
	}
	if err := qdrantJSON(ctx, cfg, http.MethodPut, "/collections/"+url.PathEscape(collection), payload, nil); err != nil {
		t.Fatalf("create Qdrant collection %q: %v", collection, err)
	}
}

func deleteQdrantCollection(ctx context.Context, cfg knowledgeSmokeConfig, collection string) error {
	return qdrantJSON(ctx, cfg, http.MethodDelete, "/collections/"+url.PathEscape(collection), nil, nil)
}

func retrieveQdrantPointPayload(t *testing.T, ctx context.Context, cfg knowledgeSmokeConfig, collection string, pointID string) map[string]any {
	t.Helper()
	var decoded struct {
		Result []struct {
			ID      any            `json:"id"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	payload := map[string]any{
		"ids":          []string{pointID},
		"with_payload": true,
		"with_vector":  false,
	}
	if err := qdrantJSON(ctx, cfg, http.MethodPost, "/collections/"+url.PathEscape(collection)+"/points", payload, &decoded); err != nil {
		t.Fatalf("retrieve Qdrant point %q: %v", pointID, err)
	}
	if len(decoded.Result) != 1 {
		t.Fatalf("Qdrant point lookup returned %d result(s), want 1", len(decoded.Result))
	}
	if decoded.Result[0].Payload == nil {
		t.Fatal("Qdrant point payload is empty")
	}
	return decoded.Result[0].Payload
}

func qdrantJSON(ctx context.Context, cfg knowledgeSmokeConfig, method string, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode Qdrant request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.qdrantURL+path, body)
	if err != nil {
		return fmt.Errorf("build Qdrant request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.qdrantAPIKey != "" {
		req.Header.Set("api-key", cfg.qdrantAPIKey)
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Qdrant request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		return fmt.Errorf("Qdrant returned HTTP %d", res.StatusCode)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1024))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode Qdrant response: %w", err)
	}
	return nil
}

func assertChunkIndexed(t *testing.T, chunk service.DocumentChunk, cfg knowledgeSmokeConfig) {
	t.Helper()
	if strings.TrimSpace(chunk.ID) == "" {
		t.Fatal("chunk id is empty")
	}
	if chunk.QdrantPointID == nil || strings.TrimSpace(*chunk.QdrantPointID) == "" {
		t.Fatalf("chunk %q has no Qdrant point id", chunk.ID)
	}
	if chunk.EmbeddingProvider == nil || strings.TrimSpace(*chunk.EmbeddingProvider) == "" {
		t.Fatalf("chunk %q has no embedding provider", chunk.ID)
	}
	if chunk.EmbeddingModel == nil || strings.TrimSpace(*chunk.EmbeddingModel) == "" {
		t.Fatalf("chunk %q has no embedding model", chunk.ID)
	}
	if chunk.EmbeddingDimension == nil || *chunk.EmbeddingDimension <= 0 {
		t.Fatalf("chunk %q embedding dimension = %v, want > 0", chunk.ID, chunk.EmbeddingDimension)
	}
	if !strings.EqualFold(cfg.embeddingProvider, "ai_gateway") && int(*chunk.EmbeddingDimension) != cfg.embeddingDimension {
		t.Fatalf("chunk %q embedding dimension = %d, want %d", chunk.ID, *chunk.EmbeddingDimension, cfg.embeddingDimension)
	}
}

func assertQdrantPayload(t *testing.T, payload map[string]any, knowledgeBaseID string, documentID string, chunkID string) {
	t.Helper()
	for key, want := range map[string]string{
		"knowledge_base_id": knowledgeBaseID,
		"document_id":       documentID,
		"chunk_id":          chunkID,
	} {
		if got := strings.TrimSpace(fmt.Sprint(payload[key])); got != want {
			t.Fatalf("Qdrant payload %s = %q, want %q; payload keys = %v", key, got, want, sortedMapKeys(payload))
		}
	}
	if _, ok := payload["chunk_index"]; !ok {
		t.Fatalf("Qdrant payload missing chunk_index; payload keys = %v", sortedMapKeys(payload))
	}
}

func positiveIntEnv(t *testing.T, key string, fallback int) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		t.Fatalf("%s must be a positive integer", key)
	}
	return value
}

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func stringPtr(value string) *string {
	return &value
}

func newSmokeRunID(t *testing.T) string {
	t.Helper()
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Fatalf("generate smoke run id: %v", err)
	}
	return time.Now().UTC().Format("20060102150405") + "_" + hex.EncodeToString(buf[:])
}

func qdrantCollectionName(prefix string, runID string) string {
	prefix = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(strings.TrimSpace(prefix), "_")
	prefix = strings.Trim(prefix, "_-")
	if prefix == "" {
		prefix = "knowledge_ingestion_smoke"
	}
	return prefix + "_" + safeIdentifierSuffix(runID)
}

func safeIdentifierSuffix(value string) string {
	value = regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(strings.TrimSpace(value), "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "run"
	}
	return strings.ToLower(value)
}

func sortedMapKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
