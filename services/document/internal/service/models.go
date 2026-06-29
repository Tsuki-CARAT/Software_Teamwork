package service

import "time"

type ReportStatus string

const (
	ReportStatusDraft ReportStatus = "draft"
)

type JobType string

const (
	JobTypeOutlineGeneration JobType = "outline_generation"
)

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
)

type ReportType struct {
	Code              string
	Name              string
	Description       string
	Enabled           bool
	DefaultTemplateID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Report struct {
	ID                 string
	Name               string
	ReportType         string
	TemplateID         string
	Topic              string
	Specialty          string
	BusinessObject     string
	Year               int
	Status             ReportStatus
	CreatorID          string
	CreatorName        string
	Source             string
	LatestJobID        string
	LatestReportFileID string
	GeneratedAt        *time.Time
	ExportedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

type ReportJob struct {
	ID           string
	RequestID    string
	Source       string
	JobType      JobType
	TargetType   string
	TargetID     string
	AsynqTaskID  string
	QueueName    string
	ReportID     string
	TemplateID   string
	Status       JobStatus
	ErrorCode    string
	ErrorMessage string
	RetryCount   int
	MaxAttempts  int
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
}

type ReportJobAttempt struct {
	ID            string
	JobID         string
	AttemptNumber int
	AsynqTaskID   string
	TriggerSource string
	Reason        string
	Status        JobStatus
	ErrorCode     string
	ErrorMessage  string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
}

type ReportEvent struct {
	ID        string
	ReportID  string
	JobID     string
	EventType string
	Message   string
	CreatedAt time.Time
}
