package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/platform/aigatewayclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/document/internal/service"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TaskOutlineGeneration   = "document:report:outline_generation"
	TaskOutlineRegeneration = "document:report:outline_regeneration"
	TaskContentGeneration   = "document:report:content_generation"
	TaskContentRegeneration = "document:report:content_regeneration"
	TaskSectionRegeneration = "document:report:section_regeneration"
	TaskReportFileCreation  = "document:report:report_file_creation"
)

type ReportJobPayload struct {
	RequestID string `json:"requestId"`
	JobType   string `json:"jobType"`
	JobID     string `json:"jobId"`
	AttemptID string `json:"attemptId"`
	UserID    string `json:"userId"`
}

func TaskTypeForJobType(jobType service.JobType) (string, error) {
	switch jobType {
	case service.JobTypeOutlineGeneration:
		return TaskOutlineGeneration, nil
	case service.JobTypeOutlineRegeneration:
		return TaskOutlineRegeneration, nil
	case service.JobTypeContentGeneration:
		return TaskContentGeneration, nil
	case service.JobTypeContentRegeneration:
		return TaskContentRegeneration, nil
	case service.JobTypeSectionRegeneration:
		return TaskSectionRegeneration, nil
	case service.JobTypeReportFileCreation:
		return TaskReportFileCreation, nil
	default:
		return "", fmt.Errorf("unsupported report job type: %s", jobType)
	}
}

// GenerationRepository is the DB interface the worker needs for generation orchestration.
type GenerationRepository interface {
	// job state
	FindReportJobByID(ctx context.Context, id string) (service.ReportJob, error)
	SetJobRunning(ctx context.Context, id string) error
	SetJobSucceeded(ctx context.Context, id string) error
	SetJobPartialSucceeded(ctx context.Context, id string) error
	SetJobFailed(ctx context.Context, id, errCode, errMsg string) error
	SetAttemptRunning(ctx context.Context, attemptID string) error
	SetAttemptSucceeded(ctx context.Context, attemptID string) error
	SetAttemptFailed(ctx context.Context, attemptID, errCode, errMsg string) error

	// report
	GetReportByID(ctx context.Context, id string) (service.Report, error)
	UpdateReportStatus(ctx context.Context, id string, status service.ReportStatus) error
	FindReportTemplateByID(ctx context.Context, id string) (service.ReportTemplate, error)
	GetReportTemplateStructure(ctx context.Context, id string) (service.ReportTemplateStructure, error)

	// outline
	CreateReportOutline(ctx context.Context, value service.ReportOutline) (service.ReportOutline, error)
	ListReportOutlines(ctx context.Context, reportID string) ([]service.ReportOutline, error)

	// sections
	ListReportSections(ctx context.Context, reportID string) ([]service.ReportSection, error)
	CreateReportSection(ctx context.Context, value service.ReportSection) (service.ReportSection, error)
	UpdateReportSection(ctx context.Context, value service.ReportSection) (service.ReportSection, error)
	CreateReportSectionVersion(ctx context.Context, value service.ReportSectionVersion) (service.ReportSectionVersion, error)

	// events
	CreateReportEvent(ctx context.Context, value service.ReportEvent) (service.ReportEvent, error)

	// progress
	UpdateJobProgress(ctx context.Context, jobID string, completed, total int) error
}

type Worker struct {
	server   *asynq.Server
	mux      *asynq.ServeMux
	logger   *slog.Logger
	store    GenerationRepository
	aiClient *aigatewayclient.Client
}

func New(redisAddr string, logger *slog.Logger, store GenerationRepository, aiClient *aigatewayclient.Client) *Worker {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 4,
			Queues:      map[string]int{"document": 1},
		},
	)
	mux := asynq.NewServeMux()
	w := &Worker{server: srv, mux: mux, logger: logger, store: store, aiClient: aiClient}
	mux.HandleFunc(TaskOutlineGeneration, w.handleReportJob)
	mux.HandleFunc(TaskOutlineRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskContentGeneration, w.handleReportJob)
	mux.HandleFunc(TaskContentRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskSectionRegeneration, w.handleReportJob)
	mux.HandleFunc(TaskReportFileCreation, w.handleReportJob)
	return w
}

func (w *Worker) Start() error {
	return w.server.Start(w.mux)
}

func (w *Worker) Stop() {
	w.server.Shutdown()
}

func (w *Worker) handleReportJob(ctx context.Context, t *asynq.Task) error {
	var payload ReportJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	w.logger.InfoContext(ctx, "report job started", "job_id", payload.JobID, "attempt_id", payload.AttemptID, "job_type", payload.JobType)

	job, err := w.store.FindReportJobByID(ctx, payload.JobID)
	if err != nil {
		w.logger.ErrorContext(ctx, "find job failed", "job_id", payload.JobID, "error", err)
		return err
	}

	if err := w.store.SetJobRunning(ctx, payload.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job running failed", "job_id", payload.JobID, "error", err)
	}
	if payload.AttemptID != "" {
		if err := w.store.SetAttemptRunning(ctx, payload.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt running failed", "attempt_id", payload.AttemptID, "error", err)
		}
	}

	var execErr error
	switch service.JobType(payload.JobType) {
	case service.JobTypeOutlineGeneration, service.JobTypeOutlineRegeneration:
		execErr = w.executeOutlineGeneration(ctx, job, payload)
	case service.JobTypeContentGeneration, service.JobTypeContentRegeneration:
		execErr = w.executeContentGeneration(ctx, job, payload)
	default:
		w.logger.WarnContext(ctx, "job type not yet implemented", "job_type", payload.JobType)
	}

	if execErr != nil {
		w.logger.ErrorContext(ctx, "report job failed", "job_id", payload.JobID, "error", execErr)
		_ = w.store.SetJobFailed(ctx, payload.JobID, "generation_failed", "generation error")
		if payload.AttemptID != "" {
			_ = w.store.SetAttemptFailed(ctx, payload.AttemptID, "generation_failed", "generation error")
		}
		return nil // return nil so asynq does not auto-retry; retry is managed via ClaimRetry
	}

	w.logger.InfoContext(ctx, "report job completed", "job_id", payload.JobID, "job_type", payload.JobType)
	if err := w.store.SetJobSucceeded(ctx, payload.JobID); err != nil {
		w.logger.ErrorContext(ctx, "mark job succeeded failed", "job_id", payload.JobID, "error", err)
		if payload.AttemptID != "" {
			_ = w.store.SetAttemptFailed(ctx, payload.AttemptID, "state_error", err.Error())
		}
		return err
	}
	if payload.AttemptID != "" {
		if err := w.store.SetAttemptSucceeded(ctx, payload.AttemptID); err != nil {
			w.logger.ErrorContext(ctx, "mark attempt succeeded failed", "attempt_id", payload.AttemptID, "error", err)
		}
	}
	return nil
}

// executeOutlineGeneration calls AI Gateway to generate a report outline and saves the result.
func (w *Worker) executeOutlineGeneration(ctx context.Context, job service.ReportJob, payload ReportJobPayload) error {
	report, err := w.store.GetReportByID(ctx, job.ReportID)
	if err != nil {
		return fmt.Errorf("get report: %w", err)
	}

	_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusOutlineGenerating)
	w.emitEvent(ctx, report.ID, job.ID, "outline.generation.started", "outline generation started")

	// Read template structure if available.
	var outlineSchemaHint string
	if report.TemplateID != "" {
		if structure, err := w.store.GetReportTemplateStructure(ctx, report.TemplateID); err == nil {
			if len(structure.OutlineSchema) > 0 {
				outlineSchemaHint = string(structure.OutlineSchema)
			}
		}
	}

	messages := buildOutlinePrompt(report, outlineSchemaHint)
	rawResponse, err := w.aiClient.ChatCompletion(ctx, payload.RequestID, messages)
	if err != nil {
		_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusFailed)
		return fmt.Errorf("ai gateway chat completion: %w", err)
	}

	nodes, err := parseOutlineJSON(rawResponse)
	if err != nil {
		_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusFailed)
		return fmt.Errorf("parse outline response: %w", err)
	}
	nodes = service.RenumberOutline(nodes)

	now := time.Now().UTC()
	outline := service.ReportOutline{
		ID:          uuid.New().String(),
		ReportID:    report.ID,
		Sections:    nodes,
		Version:     1,
		Source:      service.OutlineSourceAI,
		SourceJobID: job.ID,
		IsCurrent:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Bump version if a previous outline exists.
	if existing, err := w.store.ListReportOutlines(ctx, report.ID); err == nil && len(existing) > 0 {
		outline.Version = existing[len(existing)-1].Version + 1
	}

	savedOutline, err := w.store.CreateReportOutline(ctx, outline)
	if err != nil {
		_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusFailed)
		return fmt.Errorf("save outline: %w", err)
	}

	// Create section records for each outline node.
	if err := w.createSectionsFromOutline(ctx, report.ID, savedOutline.ID, nodes, "", 0); err != nil {
		w.logger.WarnContext(ctx, "create sections from outline failed", "report_id", report.ID, "error", err)
		// non-fatal: outline is saved, sections can be retried
	}

	_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusOutlineGenerated)
	w.emitEvent(ctx, report.ID, job.ID, "outline.generation.completed", "outline generation completed")
	return nil
}

// executeContentGeneration calls AI Gateway for each section and saves results.
func (w *Worker) executeContentGeneration(ctx context.Context, job service.ReportJob, payload ReportJobPayload) error {
	report, err := w.store.GetReportByID(ctx, job.ReportID)
	if err != nil {
		return fmt.Errorf("get report: %w", err)
	}

	sections, err := w.store.ListReportSections(ctx, report.ID)
	if err != nil {
		return fmt.Errorf("list sections: %w", err)
	}
	if len(sections) == 0 {
		return fmt.Errorf("no sections found for report %s", report.ID)
	}

	_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusContentGenerating)
	_ = w.store.UpdateJobProgress(ctx, job.ID, 0, len(sections))
	w.emitEvent(ctx, report.ID, job.ID, "content.generation.started",
		fmt.Sprintf("content generation started for %d sections", len(sections)))

	succeeded := 0
	failed := 0
	for i, section := range sections {
		messages := buildSectionPrompt(report, section)
		content, err := w.aiClient.ChatCompletion(ctx, payload.RequestID, messages)
		if err != nil {
			w.logger.WarnContext(ctx, "section generation failed", "section_id", section.ID, "error", err)
			failed++
			continue
		}

		section.Content = content
		section.GenerationStatus = service.JobStatusSucceeded
		section.ContentSource = service.ContentSourceAI
		section.Version++
		now := time.Now().UTC()
		section.GeneratedAt = &now

		if _, err := w.store.UpdateReportSection(ctx, section); err != nil {
			w.logger.WarnContext(ctx, "save section content failed", "section_id", section.ID, "error", err)
			failed++
			continue
		}

		ver := service.ReportSectionVersion{
			ID:        uuid.New().String(),
			ReportID:  report.ID,
			SectionID: section.ID,
			Version:   section.Version,
			Source:    service.ContentSourceAI,
			Content:   content,
			JobID:     job.ID,
			CreatedAt: now,
		}
		if _, err := w.store.CreateReportSectionVersion(ctx, ver); err != nil {
			w.logger.WarnContext(ctx, "save section version failed", "section_id", section.ID, "error", err)
		}

		succeeded++
		_ = w.store.UpdateJobProgress(ctx, job.ID, succeeded, len(sections))
		w.emitEvent(ctx, report.ID, job.ID, "content.section.completed",
			fmt.Sprintf("section %d/%d completed: %s", i+1, len(sections), section.Title))
	}

	if succeeded == 0 {
		_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusFailed)
		_ = w.store.SetJobFailed(ctx, job.ID, "all_sections_failed", "all sections failed to generate")
		if payload.AttemptID != "" {
			_ = w.store.SetAttemptFailed(ctx, payload.AttemptID, "all_sections_failed", "all sections failed to generate")
		}
		return fmt.Errorf("all %d sections failed to generate", failed)
	}

	if failed > 0 {
		_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusGenerated)
		_ = w.store.SetJobPartialSucceeded(ctx, job.ID)
		if payload.AttemptID != "" {
			_ = w.store.SetAttemptSucceeded(ctx, payload.AttemptID)
		}
		w.emitEvent(ctx, report.ID, job.ID, "content.generation.partial",
			fmt.Sprintf("content generation partially completed: %d succeeded, %d failed", succeeded, failed))
		return nil
	}

	_ = w.store.UpdateReportStatus(ctx, report.ID, service.ReportStatusGenerated)
	w.emitEvent(ctx, report.ID, job.ID, "content.generation.completed",
		fmt.Sprintf("content generation completed: %d sections", succeeded))
	return nil
}

// createSectionsFromOutline recursively creates ReportSection rows for each outline node.
func (w *Worker) createSectionsFromOutline(ctx context.Context, reportID, outlineID string, nodes []service.ReportOutlineNode, parentID string, order int) error {
	for _, node := range nodes {
		order++
		sectionID := uuid.New().String()
		section := service.ReportSection{
			ID:               sectionID,
			ReportID:         reportID,
			OutlineID:        outlineID,
			ParentID:         parentID,
			OutlineNodeID:    node.ID,
			SectionPath:      node.Numbering,
			Title:            node.Title,
			Level:            node.Level,
			SortOrder:        order,
			Numbering:        node.Numbering,
			SectionType:      service.SectionTypeText,
			GenerationStatus: service.JobStatusPending,
			ContentSource:    service.ContentSourceManual,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		if _, err := w.store.CreateReportSection(ctx, section); err != nil {
			return fmt.Errorf("create section %s: %w", node.Numbering, err)
		}
		if len(node.Children) > 0 {
			if err := w.createSectionsFromOutline(ctx, reportID, outlineID, node.Children, sectionID, order); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Worker) emitEvent(ctx context.Context, reportID, jobID, eventType, message string) {
	event := service.ReportEvent{
		ID:        uuid.New().String(),
		ReportID:  reportID,
		JobID:     jobID,
		EventType: eventType,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := w.store.CreateReportEvent(ctx, event); err != nil {
		w.logger.WarnContext(ctx, "emit event failed", "event_type", eventType, "error", err)
	}
}

// buildOutlinePrompt constructs the AI messages for outline generation.
func buildOutlinePrompt(report service.Report, templateHint string) []aigatewayclient.ChatMessage {
	system := "You are a professional report writer specializing in the power industry. " +
		"Generate a structured multi-level outline for the given report. " +
		"Return ONLY a valid JSON array. Each element must have: " +
		`"id" (string), "title" (string), and optionally "children" (array of the same structure). ` +
		"Do not include any text outside the JSON array."

	var userParts []string
	userParts = append(userParts, fmt.Sprintf("Report type: %s", report.ReportType))
	userParts = append(userParts, fmt.Sprintf("Topic: %s", report.Topic))
	if report.Specialty != "" {
		userParts = append(userParts, fmt.Sprintf("Specialty: %s", report.Specialty))
	}
	if report.BusinessObject != "" {
		userParts = append(userParts, fmt.Sprintf("Business object: %s", report.BusinessObject))
	}
	if report.Year > 0 {
		userParts = append(userParts, fmt.Sprintf("Year: %d", report.Year))
	}
	if templateHint != "" {
		userParts = append(userParts, fmt.Sprintf("Template outline schema (reference): %s", templateHint))
	}
	userParts = append(userParts, "\nGenerate the report outline now:")

	return []aigatewayclient.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: strings.Join(userParts, "\n")},
	}
}

// buildSectionPrompt constructs the AI messages for a single section's content generation.
func buildSectionPrompt(report service.Report, section service.ReportSection) []aigatewayclient.ChatMessage {
	system := "You are a professional report writer specializing in the power industry. " +
		"Write the content for the given report section. " +
		"Be concise, professional, and factually relevant. " +
		"Return only the section content text, no headings or JSON."

	user := fmt.Sprintf(
		"Report: %s\nReport type: %s\nSection %s: %s\n\nWrite the content for this section:",
		report.Topic, report.ReportType, section.Numbering, section.Title,
	)

	return []aigatewayclient.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
}

// parseOutlineJSON extracts a JSON array from AI response text and parses it as outline nodes.
func parseOutlineJSON(raw string) ([]service.ReportOutlineNode, error) {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fence if present.
	if idx := strings.Index(raw, "```"); idx != -1 {
		raw = raw[idx+3:]
		if nl := strings.Index(raw, "\n"); nl != -1 {
			raw = raw[nl+1:]
		}
		if end := strings.LastIndex(raw, "```"); end != -1 {
			raw = raw[:end]
		}
	}
	// Find the outermost JSON array.
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in AI response")
	}
	raw = raw[start : end+1]

	var nodes []service.ReportOutlineNode
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		return nil, fmt.Errorf("parse outline JSON: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("AI returned empty outline")
	}
	// Assign UUIDs to any node that doesn't have an id.
	assignNodeIDs(nodes)
	return nodes, nil
}

func assignNodeIDs(nodes []service.ReportOutlineNode) {
	for i := range nodes {
		if nodes[i].ID == "" {
			nodes[i].ID = uuid.New().String()
		}
		if len(nodes[i].Children) > 0 {
			assignNodeIDs(nodes[i].Children)
		}
	}
}
