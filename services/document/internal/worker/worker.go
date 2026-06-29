package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskOutlineGeneration = "document:report:outline_generation"
)

type OutlinePayload struct {
	RequestID string `json:"requestId"`
	JobID     string `json:"jobId"`
	UserID    string `json:"userId"`
}

type Worker struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
}

func New(redisAddr string, logger *slog.Logger) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 4,
			Queues:      map[string]int{"document": 1},
		},
	)
	mux := asynq.NewServeMux()
	w := &Worker{server: srv, mux: mux, logger: logger}
	mux.HandleFunc(TaskOutlineGeneration, w.handleOutlineGeneration)
	return w
}

func (w *Worker) Start() error {
	return w.server.Start(w.mux)
}

func (w *Worker) Stop() {
	w.server.Shutdown()
}

func (w *Worker) handleOutlineGeneration(ctx context.Context, t *asynq.Task) error {
	var p OutlinePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	w.logger.InfoContext(ctx, "mock outline generation started", "job_id", p.JobID, "request_id", p.RequestID)
	// Mock: simulate work
	time.Sleep(100 * time.Millisecond)
	w.logger.InfoContext(ctx, "mock outline generation completed", "job_id", p.JobID)
	return nil
}
