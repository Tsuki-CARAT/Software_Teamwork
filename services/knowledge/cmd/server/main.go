package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/config"
	knowledgehttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/embedding"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/fileclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/parser"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/queue"
	vectorplatform "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/platform/vector"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connection failed", "service", "knowledge", "dependency", "postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	fileClient, err := fileclient.New(cfg.FileServiceURL, cfg.ServiceToken, nil)
	if err != nil {
		logger.Error("file client configuration failed", "service", "knowledge", "dependency", "file", "error", err)
		os.Exit(1)
	}
	documentParser, err := buildParser(cfg)
	if err != nil {
		logger.Error("parser configuration failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}
	embedder, err := buildEmbedder(cfg)
	if err != nil {
		logger.Error("embedding configuration failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}
	vectorIndex, err := buildVectorIndex(cfg)
	if err != nil {
		logger.Error("vector index configuration failed", "service", "knowledge", "dependency", "qdrant", "error", err)
		os.Exit(1)
	}

	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()
	ingestionQueue := queue.NewAsynqQueue(asynqClient)

	repo := repository.NewPostgresRepository(pool)
	knowledgeService := service.NewWithDependencies(
		repo,
		fileClient,
		ingestionQueue,
		nil,
		nil,
		service.WithProcessingPipeline(fileClient, documentParser, service.NewFixedChunker()),
		service.WithVectorIndex(embedder, vectorIndex),
	)
	handler := knowledgehttp.NewServer(knowledgeService, knowledgehttp.Config{
		ServiceVersion: cfg.ServiceVersion,
		Environment:    cfg.Environment,
		Logger:         logger,
		MaxUploadBytes: cfg.MaxUploadBytes,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	asynqServer := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 1})
	asynqMux := asynq.NewServeMux()
	ingestionHandler := worker.NewIngestionHandler(knowledgeService, worker.WithLogger(logger))
	asynqMux.HandleFunc(queue.DocumentIngestionTaskType, func(ctx context.Context, task *asynq.Task) error {
		return ingestionHandler.HandleIngestionPayload(ctx, task.Payload())
	})

	go func() {
		logger.Info("knowledge service starting", "service", "knowledge", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("knowledge service stopped unexpectedly", "service", "knowledge", "error", err)
			stop()
		}
	}()
	go func() {
		logger.Info("knowledge ingestion worker starting", "service", "knowledge", "queue", queue.DocumentIngestionTaskType)
		if err := asynqServer.Run(asynqMux); err != nil {
			logger.Error("knowledge ingestion worker stopped unexpectedly", "service", "knowledge", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	logger.Info("knowledge service shutdown started", "service", "knowledge")
	asynqServer.Shutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("knowledge service shutdown failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}
	logger.Info("knowledge service shutdown complete", "service", "knowledge")
}

func connectPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func buildParser(cfg config.Config) (service.Parser, error) {
	return parser.NewServiceClient(parser.ServiceClientConfig{
		BaseURL:      cfg.ParserServiceBaseURL,
		ServiceToken: cfg.ParserServiceToken,
		Timeout:      cfg.ParserServiceTimeout,
	})
}

func buildEmbedder(cfg config.Config) (service.Embedder, error) {
	if strings.EqualFold(strings.TrimSpace(cfg.EmbeddingProvider), "ai_gateway") {
		return embedding.NewAIGatewayClient(embedding.AIGatewayConfig{
			BaseURL:      cfg.AIGatewayBaseURL,
			Model:        cfg.EmbeddingModel,
			ProfileID:    cfg.AIGatewayProfileID,
			Dimensions:   cfg.EmbeddingDimension,
			ServiceToken: cfg.AIGatewayToken,
		})
	}
	return embedding.NewLocalHasher(cfg.EmbeddingProvider, cfg.EmbeddingModel, cfg.EmbeddingDimension), nil
}

func buildVectorIndex(cfg config.Config) (service.VectorIndex, error) {
	if strings.TrimSpace(cfg.QdrantURL) == "" {
		return vectorplatform.NewMemoryIndex(), nil
	}
	return vectorplatform.NewQdrantClient(vectorplatform.QdrantConfig{
		BaseURL:    cfg.QdrantURL,
		APIKey:     cfg.QdrantAPIKey,
		Collection: cfg.QdrantCollection,
		Dimension:  cfg.EmbeddingDimension,
	})
}
