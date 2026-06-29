package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/repository/sqlc"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool    *pgxpool.Pool
	db      sqlc.DBTX
	queries *sqlc.Queries
}

func NewPostgres(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DOCUMENT_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("DOCUMENT_DATABASE_URL is invalid")
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return NewPostgresRepository(pool), nil
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, db: pool, queries: sqlc.New(pool)}
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) CheckReady(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *PostgresRepository) WithinTx(ctx context.Context, fn func(*PostgresRepository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin document transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := &PostgresRepository{
		pool:    r.pool,
		db:      tx,
		queries: r.queries.WithTx(tx),
	}
	if err := fn(txRepo); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit document transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpsertReportType(ctx context.Context, value service.ReportType) (service.ReportType, error) {
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_types (
			code, name, description, enabled, default_template_id, created_at, updated_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, '')::uuid, $6, $7)
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			enabled = EXCLUDED.enabled,
			default_template_id = EXCLUDED.default_template_id,
			updated_at = EXCLUDED.updated_at
		RETURNING code, name, COALESCE(description, ''), enabled, COALESCE(default_template_id::text, ''), created_at, updated_at`,
		value.Code,
		value.Name,
		value.Description,
		value.Enabled,
		value.DefaultTemplateID,
		value.CreatedAt,
		value.UpdatedAt,
	)
	return scanReportType(row)
}

func (r *PostgresRepository) CreateReport(ctx context.Context, value service.Report) (service.Report, error) {
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = value.CreatedAt
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO reports (
			id, report_name, report_type, template_id, topic, specialty,
			plant_or_business_object, report_year, status, creator_id, creator_name,
			source, latest_job_id, latest_report_file_id, generated_at, exported_at,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, NULLIF($4, '')::uuid, $5, NULLIF($6, ''),
			NULLIF($7, ''), NULLIF($8, 0), $9, NULLIF($10, ''), NULLIF($11, ''),
			$12, NULLIF($13, '')::uuid, NULLIF($14, '')::uuid, $15, $16, $17, $18
		)
		RETURNING
			id::text, report_name, report_type, COALESCE(template_id::text, ''), topic,
			COALESCE(specialty, ''), COALESCE(plant_or_business_object, ''),
			COALESCE(report_year, 0), status, COALESCE(creator_id, ''),
			COALESCE(creator_name, ''), source, COALESCE(latest_job_id::text, ''),
			COALESCE(latest_report_file_id::text, ''), generated_at, exported_at,
			created_at, updated_at, deleted_at`,
		value.ID,
		value.Name,
		value.ReportType,
		value.TemplateID,
		value.Topic,
		value.Specialty,
		value.BusinessObject,
		value.Year,
		string(value.Status),
		value.CreatorID,
		value.CreatorName,
		value.Source,
		value.LatestJobID,
		value.LatestReportFileID,
		value.GeneratedAt,
		value.ExportedAt,
		value.CreatedAt,
		value.UpdatedAt,
	)
	report, err := scanReport(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.Report{}, service.NewError(service.CodeConflict, "report already exists", err)
		}
		return service.Report{}, fmt.Errorf("insert report: %w", err)
	}
	return report, nil
}

func (r *PostgresRepository) CreateReportJob(ctx context.Context, value service.ReportJob) (service.ReportJob, error) {
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.MaxAttempts == 0 {
		value.MaxAttempts = 3
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_jobs (
			id, request_id, source, job_type, target_type, target_id, asynq_task_id,
			queue_name, report_id, template_id, status, error_code, error_message,
			retry_count, max_attempts, started_at, finished_at, created_at
		)
		VALUES (
			$1, NULLIF($2, ''), $3, $4, $5, $6, NULLIF($7, ''),
			$8, $9, NULLIF($10, '')::uuid, $11, NULLIF($12, ''), NULLIF($13, ''),
			$14, $15, $16, $17, $18
		)
		RETURNING
			id::text, COALESCE(request_id, ''), source, job_type, target_type,
			target_id, COALESCE(asynq_task_id, ''), queue_name, report_id::text,
			COALESCE(template_id::text, ''), status, COALESCE(error_code, ''),
			COALESCE(error_message, ''), retry_count, max_attempts, started_at,
			finished_at, created_at`,
		value.ID,
		value.RequestID,
		value.Source,
		string(value.JobType),
		value.TargetType,
		value.TargetID,
		value.AsynqTaskID,
		value.QueueName,
		value.ReportID,
		value.TemplateID,
		string(value.Status),
		value.ErrorCode,
		value.ErrorMessage,
		value.RetryCount,
		value.MaxAttempts,
		value.StartedAt,
		value.FinishedAt,
		value.CreatedAt,
	)
	job, err := scanReportJob(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportJob{}, service.NewError(service.CodeConflict, "report job already exists", err)
		}
		return service.ReportJob{}, fmt.Errorf("insert report job: %w", err)
	}
	return job, nil
}

func (r *PostgresRepository) FindReportJobByID(ctx context.Context, id string) (service.ReportJob, error) {
	jobID, err := parseUUID(id)
	if err != nil {
		return service.ReportJob{}, service.NewError(service.CodeValidation, "invalid report job id", err)
	}
	row, err := r.queries.GetReportJobByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReportJob{}, service.NewError(service.CodeNotFound, "report job not found", err)
		}
		return service.ReportJob{}, fmt.Errorf("find report job: %w", err)
	}
	return reportJobFromSQLC(row), nil
}

func (r *PostgresRepository) CreateReportJobAttempt(ctx context.Context, value service.ReportJobAttempt) (service.ReportJobAttempt, error) {
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_job_attempts (
			id, job_id, attempt_number, asynq_task_id, trigger_source, reason,
			status, error_code, error_message, started_at, finished_at, created_at
		)
		VALUES (
			$1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''),
			$7, NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12
		)
		RETURNING
			id::text, job_id::text, attempt_number, COALESCE(asynq_task_id, ''),
			trigger_source, COALESCE(reason, ''), status, COALESCE(error_code, ''),
			COALESCE(error_message, ''), started_at, finished_at, created_at`,
		value.ID,
		value.JobID,
		value.AttemptNumber,
		value.AsynqTaskID,
		value.TriggerSource,
		value.Reason,
		string(value.Status),
		value.ErrorCode,
		value.ErrorMessage,
		value.StartedAt,
		value.FinishedAt,
		value.CreatedAt,
	)
	attempt, err := scanReportJobAttempt(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportJobAttempt{}, service.NewError(service.CodeConflict, "report job attempt already exists", err)
		}
		return service.ReportJobAttempt{}, fmt.Errorf("insert report job attempt: %w", err)
	}
	return attempt, nil
}

func (r *PostgresRepository) CreateReportEvent(ctx context.Context, value service.ReportEvent) (service.ReportEvent, error) {
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO report_events (
			id, report_id, job_id, event_type, message, created_at
		)
		VALUES ($1, $2, NULLIF($3, '')::uuid, $4, NULLIF($5, ''), $6)
		RETURNING id::text, report_id::text, COALESCE(job_id::text, ''), event_type, COALESCE(message, ''), created_at`,
		value.ID,
		value.ReportID,
		value.JobID,
		value.EventType,
		value.Message,
		value.CreatedAt,
	)
	event, err := scanReportEvent(row)
	if err != nil {
		if isUniqueViolation(err) {
			return service.ReportEvent{}, service.NewError(service.CodeConflict, "report event already exists", err)
		}
		return service.ReportEvent{}, fmt.Errorf("insert report event: %w", err)
	}
	return event, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReportType(row scanner) (service.ReportType, error) {
	var value service.ReportType
	if err := row.Scan(
		&value.Code,
		&value.Name,
		&value.Description,
		&value.Enabled,
		&value.DefaultTemplateID,
		&value.CreatedAt,
		&value.UpdatedAt,
	); err != nil {
		return service.ReportType{}, err
	}
	return value, nil
}

func scanReport(row scanner) (service.Report, error) {
	var value service.Report
	var status string
	if err := row.Scan(
		&value.ID,
		&value.Name,
		&value.ReportType,
		&value.TemplateID,
		&value.Topic,
		&value.Specialty,
		&value.BusinessObject,
		&value.Year,
		&status,
		&value.CreatorID,
		&value.CreatorName,
		&value.Source,
		&value.LatestJobID,
		&value.LatestReportFileID,
		&value.GeneratedAt,
		&value.ExportedAt,
		&value.CreatedAt,
		&value.UpdatedAt,
		&value.DeletedAt,
	); err != nil {
		return service.Report{}, err
	}
	value.Status = service.ReportStatus(status)
	return value, nil
}

func scanReportJob(row scanner) (service.ReportJob, error) {
	var value service.ReportJob
	var jobType, status string
	if err := row.Scan(
		&value.ID,
		&value.RequestID,
		&value.Source,
		&jobType,
		&value.TargetType,
		&value.TargetID,
		&value.AsynqTaskID,
		&value.QueueName,
		&value.ReportID,
		&value.TemplateID,
		&status,
		&value.ErrorCode,
		&value.ErrorMessage,
		&value.RetryCount,
		&value.MaxAttempts,
		&value.StartedAt,
		&value.FinishedAt,
		&value.CreatedAt,
	); err != nil {
		return service.ReportJob{}, err
	}
	value.JobType = service.JobType(jobType)
	value.Status = service.JobStatus(status)
	return value, nil
}

func scanReportJobAttempt(row scanner) (service.ReportJobAttempt, error) {
	var value service.ReportJobAttempt
	var status string
	if err := row.Scan(
		&value.ID,
		&value.JobID,
		&value.AttemptNumber,
		&value.AsynqTaskID,
		&value.TriggerSource,
		&value.Reason,
		&status,
		&value.ErrorCode,
		&value.ErrorMessage,
		&value.StartedAt,
		&value.FinishedAt,
		&value.CreatedAt,
	); err != nil {
		return service.ReportJobAttempt{}, err
	}
	value.Status = service.JobStatus(status)
	return value, nil
}

func scanReportEvent(row scanner) (service.ReportEvent, error) {
	var value service.ReportEvent
	if err := row.Scan(
		&value.ID,
		&value.ReportID,
		&value.JobID,
		&value.EventType,
		&value.Message,
		&value.CreatedAt,
	); err != nil {
		return service.ReportEvent{}, err
	}
	return value, nil
}

func reportJobFromSQLC(row sqlc.GetReportJobByIDRow) service.ReportJob {
	return service.ReportJob{
		ID:           uuidToString(row.ID),
		RequestID:    textToString(row.RequestID),
		Source:       row.Source,
		JobType:      service.JobType(row.JobType),
		TargetType:   row.TargetType,
		TargetID:     row.TargetID,
		AsynqTaskID:  textToString(row.AsynqTaskID),
		QueueName:    row.QueueName,
		ReportID:     uuidToString(row.ReportID),
		TemplateID:   uuidToString(row.TemplateID),
		Status:       service.JobStatus(row.Status),
		ErrorCode:    textToString(row.ErrorCode),
		ErrorMessage: textToString(row.ErrorMessage),
		RetryCount:   int(row.RetryCount),
		MaxAttempts:  int(row.MaxAttempts),
		StartedAt:    timestamptzToTimePtr(row.StartedAt),
		FinishedAt:   timestamptzToTimePtr(row.FinishedAt),
		CreatedAt:    timestamptzToTime(row.CreatedAt),
	}
}

func parseUUID(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return uuid, nil
}

func uuidToString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	b := value.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func textToString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func timestamptzToTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func timestamptzToTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value") || strings.Contains(err.Error(), "SQLSTATE 23505")
}
