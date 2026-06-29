package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hibiken/asynq"
)

const (
	TaskOutlineGeneration = "document:report:outline_generation"
)

type OutlinePayload struct {
	RequestID string `json:"requestId"`
	JobID     string `json:"jobId"`
	AttemptID string `json:"attemptId"`
	UserID    string `json:"userId"`
}

// JobStateManager updates job and attempt status in the database as the worker processes tasks.
type JobStateManager interface {
	SetJobRunning(ctx context.Context, id string) error
	SetJobSucceeded(ctx context.Context, id string) error
	SetJobFailed(ctx context.Context, id, errCode, errMsg string) error
	SetAttemptRunning(ctx context.Context, attemptID string) error
	SetAttemptSucceeded(ctx context.Context, attemptID string) error
	SetAttemptFailed(ctx context.Context, attemptID, errCode, errMsg string) error
}

type Worker struct {
	server  *asynq.Server
	mux     *asynq.ServeMux
	logger  *slog.Logger
	jobsMgr JobStateManager
}

func New(redisAddr string, logger *slog.Logger, mgr JobStateManager) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 4,
			Queues:      map[string]int{"document": 1},
		},
	)
	mux := asynq.NewServeMux()
	w := &Worker{server: srv, mux: mux, logger: logger, jobsMgr: mgr}
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
	w.logger.InfoContext(ctx, "outline generation started", "job_id", p.JobID, "attempt_id", p.AttemptID)

	if err := w.jobsMgr.SetJobRunning(ctx, p.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job running failed", "job_id", p.JobID, "error", err)
	}
	if p.AttemptID != "" {
		if err := w.jobsMgr.SetAttemptRunning(ctx, p.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt running failed", "attempt_id", p.AttemptID, "error", err)
		}
	}

	// Mock: AI call placeholder — always succeeds in this version.
	w.logger.InfoContext(ctx, "outline generation completed", "job_id", p.JobID)

	if err := w.jobsMgr.SetJobSucceeded(ctx, p.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job succeeded failed", "job_id", p.JobID, "error", err)
		if p.AttemptID != "" {
			_ = w.jobsMgr.SetAttemptFailed(ctx, p.AttemptID, "state_error", err.Error())
		}
		return err
	}
	if p.AttemptID != "" {
		if err := w.jobsMgr.SetAttemptSucceeded(ctx, p.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt succeeded failed", "attempt_id", p.AttemptID, "error", err)
		}
	}
	return nil
}
