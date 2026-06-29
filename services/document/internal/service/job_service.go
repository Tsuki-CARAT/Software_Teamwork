package service

import (
	"context"
	"fmt"
	"time"
)

type JobRepository interface {
	FindReportJobByID(ctx context.Context, id string) (ReportJob, error)
	ListReportJobsByReportID(ctx context.Context, reportID string) ([]ReportJob, error)
	CreateReportJob(ctx context.Context, value ReportJob) (ReportJob, error)
	UpdateReportJobStatus(ctx context.Context, id string, status JobStatus, errorCode, errorMessage string, startedAt, finishedAt *time.Time) (ReportJob, error)
	UpdateJobAsynqTaskID(ctx context.Context, id, taskID string) error
	IncrementJobRetryCount(ctx context.Context, id string) (ReportJob, error)
	CreateReportJobAttempt(ctx context.Context, value ReportJobAttempt) (ReportJobAttempt, error)
	ListReportJobAttemptsByJobID(ctx context.Context, jobID string) ([]ReportJobAttempt, error)
	ListReportEventsByReportID(ctx context.Context, reportID string) ([]ReportEvent, error)
}

// TaskEnqueuer submits async tasks to the queue.
type TaskEnqueuer interface {
	EnqueueOutlineGeneration(ctx context.Context, jobID, requestID, userID string) (string, error)
}

type JobService struct {
	repo     JobRepository
	enqueuer TaskEnqueuer
}

func NewJobService(repo JobRepository, enqueuer TaskEnqueuer) *JobService {
	return &JobService{repo: repo, enqueuer: enqueuer}
}

func (s *JobService) GetJob(ctx context.Context, id string) (ReportJob, error) {
	return s.repo.FindReportJobByID(ctx, id)
}

func (s *JobService) ListJobs(ctx context.Context, reportID string) ([]ReportJob, error) {
	return s.repo.ListReportJobsByReportID(ctx, reportID)
}

type CreateJobInput struct {
	RequestID string
	UserID    string
	ReportID  string
	JobType   JobType
}

func (s *JobService) CreateJob(ctx context.Context, input CreateJobInput) (ReportJob, error) {
	if input.JobType != JobTypeOutlineGeneration {
		return ReportJob{}, ValidationError(map[string]string{
			"jobType": "only outline_generation is supported in this version",
		})
	}
	now := time.Now().UTC()
	job := ReportJob{
		ID:          newID(),
		RequestID:   input.RequestID,
		Source:      "api",
		JobType:     input.JobType,
		TargetType:  "report",
		TargetID:    input.ReportID,
		QueueName:   "document",
		ReportID:    input.ReportID,
		Status:      JobStatusPending,
		MaxAttempts: 3,
		CreatedAt:   now,
	}
	created, err := s.repo.CreateReportJob(ctx, job)
	if err != nil {
		return ReportJob{}, fmt.Errorf("create report job: %w", err)
	}
	taskID, err := s.enqueuer.EnqueueOutlineGeneration(ctx, created.ID, input.RequestID, input.UserID)
	if err != nil {
		return ReportJob{}, fmt.Errorf("enqueue job task: %w", err)
	}
	if err := s.repo.UpdateJobAsynqTaskID(ctx, created.ID, taskID); err != nil {
		return created, nil
	}
	created.AsynqTaskID = taskID
	return created, nil
}

func (s *JobService) RetryJob(ctx context.Context, id string) (ReportJobAttempt, error) {
	job, err := s.repo.FindReportJobByID(ctx, id)
	if err != nil {
		return ReportJobAttempt{}, err
	}
	if job.Status != JobStatusFailed && job.Status != JobStatusCanceled {
		return ReportJobAttempt{}, NewError(CodeValidation, "only failed or canceled jobs can be retried", nil)
	}
	if job.RetryCount >= job.MaxAttempts {
		return ReportJobAttempt{}, NewError(CodeValidation, "max retry attempts reached", nil)
	}
	attempt := ReportJobAttempt{
		ID:            newID(),
		JobID:         job.ID,
		AttemptNumber: job.RetryCount + 1,
		TriggerSource: "user",
		Status:        JobStatusPending,
		CreatedAt:     time.Now().UTC(),
	}
	attempt, err = s.repo.CreateReportJobAttempt(ctx, attempt)
	if err != nil {
		return ReportJobAttempt{}, fmt.Errorf("create retry attempt: %w", err)
	}
	taskID, err := s.enqueuer.EnqueueOutlineGeneration(ctx, job.ID, job.RequestID, "")
	if err != nil {
		return ReportJobAttempt{}, fmt.Errorf("enqueue retry task: %w", err)
	}
	_ = taskID
	if _, err = s.repo.IncrementJobRetryCount(ctx, id); err != nil {
		return ReportJobAttempt{}, fmt.Errorf("increment retry count: %w", err)
	}
	return attempt, nil
}

func (s *JobService) ListAttempts(ctx context.Context, jobID string) ([]ReportJobAttempt, error) {
	return s.repo.ListReportJobAttemptsByJobID(ctx, jobID)
}

func (s *JobService) ListEvents(ctx context.Context, reportID string) ([]ReportEvent, error) {
	return s.repo.ListReportEventsByReportID(ctx, reportID)
}

