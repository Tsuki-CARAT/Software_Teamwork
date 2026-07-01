package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMCPToolServiceListToolsDefinesStableSchemas(t *testing.T) {
	svc := NewMCPToolService(MCPToolServiceConfig{})

	tools := svc.ListTools(context.Background())
	want := []string{
		DocumentMCPToolGenerateReportOutline,
		DocumentMCPToolRegenerateReportOutline,
		DocumentMCPToolGenerateReportText,
		DocumentMCPToolRegenerateReportText,
		DocumentMCPToolRegenerateReportSection,
		DocumentMCPToolGetGenerationStatus,
		DocumentMCPToolGetTemplateSchema,
		DocumentMCPToolExportReportDOCX,
		DocumentMCPToolGetReportResult,
	}
	if len(tools) != len(want) {
		t.Fatalf("tool count = %d, want %d", len(tools), len(want))
	}
	seen := map[string]MCPToolDefinition{}
	for _, tool := range tools {
		if tool.Name == "" || tool.Description == "" {
			t.Fatalf("tool has empty name or description: %+v", tool)
		}
		if tool.InputSchema["type"] != "object" || tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("tool %s schema = %+v, want strict object", tool.Name, tool.InputSchema)
		}
		seen[tool.Name] = tool
	}
	for _, name := range want {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing tool %q from registry", name)
		}
	}
	assertSchemaRequires(t, seen[DocumentMCPToolGenerateReportOutline].InputSchema, "reportId")
	assertSchemaRequires(t, seen[DocumentMCPToolRegenerateReportSection].InputSchema, "reportId", "sectionId")
	assertSchemaRequires(t, seen[DocumentMCPToolGetGenerationStatus].InputSchema, "jobId")
	assertSchemaRequires(t, seen[DocumentMCPToolGetTemplateSchema].InputSchema, "templateId")
	assertSchemaRequires(t, seen[DocumentMCPToolExportReportDOCX].InputSchema, "reportId")
	assertSchemaRequires(t, seen[DocumentMCPToolGetReportResult].InputSchema, "reportId")
}

func TestMCPToolServiceCreateGenerationJobMapsInputsAndLogsSafeSummary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	jobs := &fakeMCPJobService{
		createJob: ReportJob{
			ID:         "job-1",
			ReportID:   "report-1",
			JobType:    JobTypeSectionRegeneration,
			TargetType: "section",
			TargetID:   "section-1",
			Status:     JobStatusPending,
			CreatedAt:  now,
		},
	}
	recorder := &fakeMCPOperationRecorder{}
	svc := NewMCPToolService(MCPToolServiceConfig{JobService: jobs, Recorder: recorder})
	svc.now = func() time.Time { return now }

	result := svc.CallTool(ctx, RequestContext{UserID: "user-1", RequestID: "req-mcp-1"},
		DocumentMCPToolRegenerateReportSection,
		json.RawMessage(`{
			"reportId":"report-1",
			"sectionId":"section-1",
			"requirements":"include https://minio.local/private prompt=secret",
			"materialIds":["material-1"," ","material-2"],
			"options":{"temperature":0.2},
			"retrieval":{"topK":3}
		}`))

	if result.Status != documentMCPToolResultAccepted || result.Error != nil {
		t.Fatalf("CallTool() result = %+v, want accepted", result)
	}
	if result.Job == nil || result.Job.ID != "job-1" || result.Job.TargetType != "section" || result.Job.TargetID != "section-1" {
		t.Fatalf("job summary = %+v", result.Job)
	}
	input := jobs.createInputs[0]
	if input.JobType != JobTypeSectionRegeneration || input.TargetScope != "section" || input.SectionID != "section-1" {
		t.Fatalf("CreateJob input target = %+v", input)
	}
	if input.Requirements == "" || len(input.MaterialIDs) != 2 || input.Options["temperature"] != float64(0.2) {
		t.Fatalf("CreateJob input payload = %+v", input)
	}
	log := recorder.singleLog(t)
	if log.OperationType != OperationDocumentMCPToolCall ||
		log.RequestSource != documentMCPRequestSource ||
		log.ToolName != DocumentMCPToolRegenerateReportSection ||
		log.OperationResult != OperationResultSucceeded ||
		log.TargetType != "job" ||
		log.TargetID != "job-1" {
		t.Fatalf("operation log = %+v", log)
	}
	summary := log.ParameterSummary
	if _, ok := summary["requirements"]; ok {
		t.Fatalf("operation log leaked requirements text: %+v", summary)
	}
	if summary["requirementsLength"] == 0 || summary["materialCount"] != 2 || summary["optionsProvided"] != true || summary["retrievalProvided"] != true {
		t.Fatalf("operation log summary = %+v", summary)
	}
}

func TestMCPToolServiceValidationAndErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		service  *MCPToolService
		toolName string
		args     string
		wantCode string
	}{
		{
			name:     "missing report id",
			service:  NewMCPToolService(MCPToolServiceConfig{JobService: &fakeMCPJobService{}, Recorder: &fakeMCPOperationRecorder{}}),
			toolName: DocumentMCPToolGenerateReportOutline,
			args:     `{}`,
			wantCode: string(CodeValidation),
		},
		{
			name:     "invalid arguments shape",
			service:  NewMCPToolService(MCPToolServiceConfig{Recorder: &fakeMCPOperationRecorder{}}),
			toolName: DocumentMCPToolGetReportResult,
			args:     `[]`,
			wantCode: string(CodeValidation),
		},
		{
			name:     "unknown tool",
			service:  NewMCPToolService(MCPToolServiceConfig{Recorder: &fakeMCPOperationRecorder{}}),
			toolName: "delete_everything",
			args:     `{}`,
			wantCode: documentMCPErrorUnsupported,
		},
		{
			name: "forbidden job status",
			service: NewMCPToolService(MCPToolServiceConfig{
				JobService: &fakeMCPJobService{getErr: NewError(CodeForbidden, "report access denied", nil)},
				Recorder:   &fakeMCPOperationRecorder{},
			}),
			toolName: DocumentMCPToolGetGenerationStatus,
			args:     `{"jobId":"job-1"}`,
			wantCode: string(CodeForbidden),
		},
		{
			name: "dependency error",
			service: NewMCPToolService(MCPToolServiceConfig{
				JobService: &fakeMCPJobService{getErr: errors.New("postgres unavailable")},
				Recorder:   &fakeMCPOperationRecorder{},
			}),
			toolName: DocumentMCPToolGetGenerationStatus,
			args:     `{"jobId":"job-1"}`,
			wantCode: string(CodeInternal),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.service.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-error"}, tt.toolName, json.RawMessage(tt.args))
			if result.Status != documentMCPToolResultFailed || result.Error == nil || result.Error.Code != tt.wantCode {
				t.Fatalf("CallTool() result = %+v, want failed %s", result, tt.wantCode)
			}
		})
	}
}

func TestMCPToolServiceRejectsInvalidGenerationArgumentTypes(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		wantField string
	}{
		{
			name:      "material ids must be string array",
			args:      `{"reportId":"report-1","materialIds":["material-1",3]}`,
			wantField: "materialIds",
		},
		{
			name:      "options must be object",
			args:      `{"reportId":"report-1","options":["not-object"]}`,
			wantField: "options",
		},
		{
			name:      "retrieval must be object",
			args:      `{"reportId":"report-1","retrieval":"not-object"}`,
			wantField: "retrieval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := &fakeMCPJobService{}
			result := NewMCPToolService(MCPToolServiceConfig{
				JobService: jobs,
				Recorder:   &fakeMCPOperationRecorder{},
			}).CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-invalid-type"},
				DocumentMCPToolGenerateReportOutline, json.RawMessage(tt.args))

			if result.Status != documentMCPToolResultFailed || result.Error == nil || result.Error.Code != string(CodeValidation) {
				t.Fatalf("CallTool() result = %+v, want validation failure", result)
			}
			if result.Error.Fields[tt.wantField] == "" {
				t.Fatalf("validation fields = %+v, missing %q", result.Error.Fields, tt.wantField)
			}
			if len(jobs.createInputs) != 0 {
				t.Fatalf("invalid arguments should not call CreateJob, inputs=%+v", jobs.createInputs)
			}
		})
	}
}

func TestMCPToolServiceGetTemplateSchemaReturnsSafeStructure(t *testing.T) {
	svc := NewMCPToolService(MCPToolServiceConfig{
		DocumentService: &fakeMCPDocumentService{
			structure: ReportTemplateStructure{
				OutlineSchema: json.RawMessage(`{"type":"object","properties":{"sections":{"type":"array"}}}`),
				StyleConfig:   json.RawMessage(`{"numbering":"global"}`),
			},
		},
		Recorder: &fakeMCPOperationRecorder{},
	})

	result := svc.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-template"},
		DocumentMCPToolGetTemplateSchema, json.RawMessage(`{"templateId":"tpl-1"}`))

	if result.Status != documentMCPToolResultSucceeded || result.TemplateSchema == nil {
		t.Fatalf("CallTool() result = %+v, want template schema", result)
	}
	if result.TemplateSchema.TemplateID != "tpl-1" || !strings.Contains(string(result.TemplateSchema.OutlineSchema), "sections") {
		t.Fatalf("template schema = %+v", result.TemplateSchema)
	}
}

func TestMCPToolServiceGetGenerationStatusSanitizesProgress(t *testing.T) {
	svc := NewMCPToolService(MCPToolServiceConfig{
		JobService: &fakeMCPJobService{getJob: ReportJob{
			ID:       "job-1",
			ReportID: "report-1",
			JobType:  JobTypeContentGeneration,
			Status:   JobStatusRunning,
			Progress: map[string]any{
				"percent": 40,
				"prompt":  "raw prompt must not survive",
				"detail":  "https://minio.local/object",
			},
		}},
		Recorder: &fakeMCPOperationRecorder{},
	})

	result := svc.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-status"},
		DocumentMCPToolGetGenerationStatus, json.RawMessage(`{"jobId":"job-1"}`))

	if result.Status != documentMCPToolResultSucceeded || result.Job == nil {
		t.Fatalf("CallTool() result = %+v, want job status", result)
	}
	if _, ok := result.Job.Progress["prompt"]; ok {
		t.Fatalf("job progress leaked prompt: %+v", result.Job.Progress)
	}
	if got := result.Job.Progress["detail"]; got != "[redacted]" {
		t.Fatalf("job progress detail = %v, want redacted", got)
	}
}

func TestMCPToolServiceExportReportDOCXUsesBasicExporterAndHidesFileRef(t *testing.T) {
	files := &fakeMCPReportFileService{
		createFile: ReportFile{
			ID:        "rf-1",
			ReportID:  "report-1",
			JobID:     "job-file-1",
			Filename:  "Report.docx",
			Format:    ReportFileFormatDOCX,
			FileRef:   "file_ref_internal_secret",
			FileSize:  1024,
			Status:    ReportFileStatusPending,
			CreatedAt: time.Date(2026, 7, 1, 9, 5, 0, 0, time.UTC),
		},
	}
	recorder := &fakeMCPOperationRecorder{}
	svc := NewMCPToolService(MCPToolServiceConfig{ReportFileSvc: files, Recorder: recorder})

	result := svc.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-export"},
		DocumentMCPToolExportReportDOCX,
		json.RawMessage(`{"reportId":"report-1","templateId":"tpl-1","format":"docx","exportOptions":{"numbering":"global"}}`))

	if result.Status != documentMCPToolResultAccepted || result.ReportFile == nil {
		t.Fatalf("CallTool() result = %+v, want accepted report file", result)
	}
	if files.createInputs[0].Format != ReportFileFormatDOCX || string(files.createInputs[0].StyleOptions) == "" {
		t.Fatalf("CreateReportFile input = %+v", files.createInputs[0])
	}
	if result.ReportFile.ContentPath != "/api/v1/report-files/rf-1/content" {
		t.Fatalf("content path = %q", result.ReportFile.ContentPath)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "file_ref") || strings.Contains(string(raw), "internal_secret") {
		t.Fatalf("tool result leaked File internal reference: %s", raw)
	}
	log := recorder.singleLog(t)
	if log.ParameterSummary["basicDocxExporter"] != true || log.ParameterSummary["richDocxRequested"] != false {
		t.Fatalf("export summary = %+v", log.ParameterSummary)
	}
}

func TestMCPToolServiceExportReportDOCXRejectsUnsupportedFormat(t *testing.T) {
	files := &fakeMCPReportFileService{}
	svc := NewMCPToolService(MCPToolServiceConfig{ReportFileSvc: files, Recorder: &fakeMCPOperationRecorder{}})

	result := svc.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-export"},
		DocumentMCPToolExportReportDOCX, json.RawMessage(`{"reportId":"report-1","format":"pdf"}`))

	if result.Status != documentMCPToolResultFailed || result.Error == nil || result.Error.Code != string(CodeValidation) {
		t.Fatalf("CallTool() result = %+v, want validation failure", result)
	}
	if len(files.createInputs) != 0 {
		t.Fatalf("unsupported export should not call service, inputs=%+v", files.createInputs)
	}
}

func TestMCPToolServiceExportAndResultErrorsUseStableCodes(t *testing.T) {
	tests := []struct {
		name     string
		service  *MCPToolService
		toolName string
		args     string
		wantCode Code
	}{
		{
			name: "export conflict",
			service: NewMCPToolService(MCPToolServiceConfig{
				ReportFileSvc: &fakeMCPReportFileService{createErr: NewError(CodeConflict, "report file is not ready", nil)},
				Recorder:      &fakeMCPOperationRecorder{},
			}),
			toolName: DocumentMCPToolExportReportDOCX,
			args:     `{"reportId":"report-1"}`,
			wantCode: CodeConflict,
		},
		{
			name: "export dependency",
			service: NewMCPToolService(MCPToolServiceConfig{
				ReportFileSvc: &fakeMCPReportFileService{createErr: NewError(CodeDependency, "redis unavailable", nil)},
				Recorder:      &fakeMCPOperationRecorder{},
			}),
			toolName: DocumentMCPToolExportReportDOCX,
			args:     `{"reportId":"report-1"}`,
			wantCode: CodeDependency,
		},
		{
			name: "result latest file dependency",
			service: NewMCPToolService(MCPToolServiceConfig{
				ReportService: &fakeMCPReportService{report: Report{
					ID:                 "report-1",
					LatestReportFileID: "rf-1",
					Status:             ReportStatusGenerated,
				}},
				ReportFileSvc: &fakeMCPReportFileService{getErr: NewError(CodeDependency, "file service unavailable", nil)},
				Recorder:      &fakeMCPOperationRecorder{},
			}),
			toolName: DocumentMCPToolGetReportResult,
			args:     `{"reportId":"report-1"}`,
			wantCode: CodeDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.service.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-stable-error"}, tt.toolName, json.RawMessage(tt.args))
			if result.Status != documentMCPToolResultFailed || result.Error == nil || result.Error.Code != string(tt.wantCode) {
				t.Fatalf("CallTool() result = %+v, want failed %s", result, tt.wantCode)
			}
		})
	}
}

func TestMCPToolServiceGetReportResultIncludesSafeLatestFile(t *testing.T) {
	generatedAt := time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC)
	svc := NewMCPToolService(MCPToolServiceConfig{
		ReportService: &fakeMCPReportService{report: Report{
			ID:                 "report-1",
			Name:               "Inspection",
			ReportType:         "summer_peak_inspection",
			TemplateID:         "tpl-1",
			Status:             ReportStatusGenerated,
			LatestJobID:        "job-1",
			LatestReportFileID: "rf-1",
			GeneratedAt:        &generatedAt,
			UpdatedAt:          generatedAt,
		}},
		ReportFileSvc: &fakeMCPReportFileService{getFile: ReportFile{
			ID:       "rf-1",
			ReportID: "report-1",
			JobID:    "job-file-1",
			Filename: "Inspection.docx",
			Format:   ReportFileFormatDOCX,
			FileRef:  "file_ref_hidden",
			Status:   ReportFileStatusSucceeded,
		}},
		Recorder: &fakeMCPOperationRecorder{},
	})

	result := svc.CallTool(context.Background(), RequestContext{UserID: "user-1", RequestID: "req-result"},
		DocumentMCPToolGetReportResult, json.RawMessage(`{"reportId":"report-1"}`))

	if result.Status != documentMCPToolResultSucceeded || result.Report == nil || result.ReportFile == nil {
		t.Fatalf("CallTool() result = %+v, want report and latest file", result)
	}
	if result.Report.ID != "report-1" || result.Report.LatestReportFileID != "rf-1" {
		t.Fatalf("report summary = %+v", result.Report)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "file_ref_hidden") {
		t.Fatalf("report result leaked File internal reference: %s", raw)
	}
}

func assertSchemaRequires(t *testing.T, schema map[string]any, fields ...string) {
	t.Helper()
	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("schema required field has unexpected type: %+v", schema["required"])
	}
	have := map[string]bool{}
	for _, value := range required {
		if text, ok := value.(string); ok {
			have[text] = true
		}
	}
	for _, field := range fields {
		if !have[field] {
			t.Fatalf("schema required = %+v, missing %q", required, field)
		}
	}
}

type fakeMCPDocumentService struct {
	structure ReportTemplateStructure
	err       error
}

func (f *fakeMCPDocumentService) GetReportTemplateStructure(context.Context, RequestContext, string) (ReportTemplateStructure, error) {
	if f.err != nil {
		return ReportTemplateStructure{}, f.err
	}
	return f.structure, nil
}

type fakeMCPJobService struct {
	createJob    ReportJob
	createErr    error
	createInputs []CreateJobInput
	getJob       ReportJob
	getErr       error
}

func (f *fakeMCPJobService) CreateJob(_ context.Context, _ RequestContext, input CreateJobInput) (ReportJob, error) {
	f.createInputs = append(f.createInputs, input)
	if f.createErr != nil {
		return ReportJob{}, f.createErr
	}
	job := f.createJob
	if job.ID == "" {
		job = ReportJob{ID: "job-1", ReportID: input.ReportID, JobType: input.JobType, TargetType: input.TargetScope, TargetID: input.SectionID, Status: JobStatusPending}
	}
	return job, nil
}

func (f *fakeMCPJobService) GetJob(context.Context, RequestContext, string) (ReportJob, error) {
	if f.getErr != nil {
		return ReportJob{}, f.getErr
	}
	if f.getJob.ID == "" {
		return ReportJob{ID: "job-1", ReportID: "report-1", JobType: JobTypeContentGeneration, Status: JobStatusRunning}, nil
	}
	return f.getJob, nil
}

type fakeMCPReportService struct {
	report Report
	err    error
}

func (f *fakeMCPReportService) GetReport(context.Context, RequestContext, string) (Report, error) {
	if f.err != nil {
		return Report{}, f.err
	}
	return f.report, nil
}

type fakeMCPReportFileService struct {
	createFile   ReportFile
	createErr    error
	createInputs []CreateReportFileInput
	getFile      ReportFile
	getErr       error
}

func (f *fakeMCPReportFileService) CreateReportFile(_ context.Context, _ RequestContext, input CreateReportFileInput) (ReportFile, error) {
	f.createInputs = append(f.createInputs, input)
	if f.createErr != nil {
		return ReportFile{}, f.createErr
	}
	return f.createFile, nil
}

func (f *fakeMCPReportFileService) GetReportFile(context.Context, RequestContext, string) (ReportFile, error) {
	if f.getErr != nil {
		return ReportFile{}, f.getErr
	}
	return f.getFile, nil
}

type fakeMCPOperationRecorder struct {
	logs []OperationLog
}

func (f *fakeMCPOperationRecorder) CreateOperationLog(_ context.Context, log OperationLog) (OperationLog, error) {
	f.logs = append(f.logs, log)
	return log, nil
}

func (f *fakeMCPOperationRecorder) singleLog(t *testing.T) OperationLog {
	t.Helper()
	if len(f.logs) != 1 {
		t.Fatalf("operation log count = %d, want 1: %+v", len(f.logs), f.logs)
	}
	return f.logs[0]
}
