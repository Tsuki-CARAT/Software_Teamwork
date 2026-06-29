package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type Code string

const (
	CodeValidation        Code = "validation_error"
	CodeUnauthorized      Code = "unauthorized"
	CodeForbidden         Code = "forbidden"
	CodeNotFound          Code = "not_found"
	CodeConflict          Code = "conflict"
	CodeDependency        Code = "dependency_error"
	CodeInternal          Code = "internal_error"
	CodeUnsupportedIntent Code = "unsupported_intent"
)

type AppError struct {
	Code    Code
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func NewError(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ValidationError(fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

func Classify(err error) (*AppError, bool) {
	var appErr *AppError
	return appErr, errors.As(err, &appErr)
}

type Conversation struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	OwnerUserID        string     `json:"-"`
	Status             string     `json:"status"`
	MessageCount       int        `json:"messageCount"`
	LastMessagePreview string     `json:"lastMessagePreview,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	LastMessageAt      *time.Time `json:"-"`
}

type Message struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"sessionId"`
	SequenceNo     int        `json:"sequenceNo"`
	Role           string     `json:"role"`
	Content        string     `json:"content"`
	Intent         string     `json:"intent,omitempty"`
	Status         string     `json:"status"`
	CitationCount  int        `json:"-"`
	CreatedAt      time.Time  `json:"createdAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

type ReasoningStep struct {
	ID        string    `json:"id,omitempty"`
	MessageID string    `json:"messageId,omitempty"`
	Type      string    `json:"type"`
	Title     string    `json:"title,omitempty"`
	Summary   string    `json:"summary"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

func (s ReasoningStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string `json:"type"`
		Label  string `json:"label,omitempty"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
	}{Type: publicStepType(s.Type), Label: s.Title, Status: publicStepStatus(s.Status), Detail: s.Summary})
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type RetrievalOptions struct {
	TopK            int                 `json:"topK,omitempty"`
	ScoreThreshold  float64             `json:"scoreThreshold,omitempty"`
	RerankThreshold float64             `json:"rerankThreshold,omitempty"`
	EnableRerank    bool                `json:"enableRerank,omitempty"`
	TagFilters      map[string][]string `json:"tagFilters,omitempty"`
}

type AskInput struct {
	Message          string           `json:"message"`
	Mode             string           `json:"mode,omitempty"`
	KnowledgeBaseIDs []string         `json:"knowledgeBaseIds,omitempty"`
	Retrieval        RetrievalOptions `json:"retrieval,omitempty"`
}

type AskResult struct {
	UserMessage      Message         `json:"userMessage"`
	AssistantMessage Message         `json:"assistantMessage"`
	ResponseRun      ResponseRun     `json:"responseRun"`
	Citations        []any           `json:"citations"`
	ReasoningSteps   []ReasoningStep `json:"reasoningSteps"`
}

type ProgressEvent struct {
	Type             string
	Sequence         int
	Payload          map[string]any
	UserMessageID    string
	AssistantMessage string
	Intent           string
	Step             ReasoningStep
}

type ProgressObserver func(ProgressEvent)

type Repository interface {
	CreateConversation(context.Context, Conversation) (Conversation, error)
	ListConversations(context.Context, string, int, int, string) (Page[Conversation], error)
	GetConversation(context.Context, string, string) (Conversation, error)
	UpdateConversation(context.Context, string, Conversation) (Conversation, error)
	DeleteConversation(context.Context, string, string) error
	ListMessages(context.Context, string, string, int, int) (Page[Message], error)
	AppendMessages(context.Context, string, string, ...Message) (ResponseRun, error)
	UpdateMessage(context.Context, string, Message) error
	SaveReasoningSteps(context.Context, string, string, []ReasoningStep) error
	SaveStreamEvents(context.Context, string, string, []StreamEvent) error
	GetResponseRun(context.Context, string, string) (ResponseRun, error)
}

type AgentRunner interface {
	RunWithObserver(context.Context, []agent.Message, agent.Observer) (agent.Result, error)
}

type RuntimeSnapshot struct {
	Runner       AgentRunner
	SystemPrompt string
}

type RuntimeProvider interface {
	Acquire() (RuntimeSnapshot, func(), error)
}

type QAService struct {
	repository Repository
	runtime    RuntimeProvider
	now        func() time.Time
	activeMu   sync.Mutex
	activeRuns map[string]context.CancelFunc
}

func NewQAService(repository Repository, runtime RuntimeProvider) (*QAService, error) {
	if repository == nil || runtime == nil {
		return nil, errors.New("repository and runtime provider are required")
	}
	return &QAService{repository: repository, runtime: runtime, now: time.Now, activeRuns: map[string]context.CancelFunc{}}, nil
}

func (s *QAService) CreateConversation(ctx context.Context, userID, title string) (Conversation, error) {
	if strings.TrimSpace(userID) == "" {
		return Conversation{}, NewError(CodeUnauthorized, "authentication required", nil)
	}
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > 200 {
		return Conversation{}, ValidationError(map[string]string{"title": "must not exceed 200 characters"})
	}
	now := s.now().UTC()
	return s.repository.CreateConversation(ctx, Conversation{
		ID: newID("conv"), Title: title, OwnerUserID: userID, Status: "active", CreatedAt: now, UpdatedAt: now,
	})
}

func (s *QAService) ListConversations(ctx context.Context, userID string, page, pageSize int, query string) (Page[Conversation], error) {
	return s.repository.ListConversations(ctx, userID, page, pageSize, query)
}

func (s *QAService) GetConversation(ctx context.Context, userID, id string) (Conversation, error) {
	return s.repository.GetConversation(ctx, userID, id)
}

func (s *QAService) UpdateConversation(ctx context.Context, userID, id, title, status string) (Conversation, error) {
	title, status = strings.TrimSpace(title), strings.TrimSpace(status)
	if title == "" && status == "" {
		return Conversation{}, ValidationError(map[string]string{"body": "title or status is required"})
	}
	if utf8.RuneCountInString(title) > 200 {
		return Conversation{}, ValidationError(map[string]string{"title": "must not exceed 200 characters"})
	}
	if status != "" && status != "active" && status != "archived" {
		return Conversation{}, ValidationError(map[string]string{"status": "must be active or archived"})
	}
	conversation, err := s.repository.GetConversation(ctx, userID, id)
	if err != nil {
		return Conversation{}, err
	}
	if title != "" {
		conversation.Title = title
	}
	if status != "" {
		conversation.Status = status
	}
	conversation.UpdatedAt = s.now().UTC()
	return s.repository.UpdateConversation(ctx, userID, conversation)
}

func (s *QAService) DeleteConversation(ctx context.Context, userID, id string) error {
	return s.repository.DeleteConversation(ctx, userID, id)
}

func (s *QAService) ListMessages(ctx context.Context, userID, conversationID string, page, pageSize int) (Page[Message], error) {
	return s.repository.ListMessages(ctx, userID, conversationID, page, pageSize)
}

func (s *QAService) Ask(ctx context.Context, userID, conversationID string, input AskInput, observe ProgressObserver) (AskResult, error) {
	if err := validateAskInput(input); err != nil {
		return AskResult{}, err
	}
	conversation, err := s.repository.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return AskResult{}, err
	}
	history, err := s.repository.ListMessages(ctx, userID, conversationID, 1, 100)
	if err != nil {
		return AskResult{}, err
	}

	now := s.now().UTC()
	intent := input.Mode
	if intent == "" {
		intent = "unknown"
	}
	userMessage := Message{ID: newID("msg"), ConversationID: conversationID, Role: agent.RoleUser, Content: strings.TrimSpace(input.Message), Intent: intent, Status: "completed", CreatedAt: now}
	assistantMessage := Message{ID: newID("msg"), ConversationID: conversationID, Role: agent.RoleAssistant, Intent: intent, Status: "generating", CreatedAt: now}
	run, err := s.repository.AppendMessages(ctx, userID, conversationID, userMessage, assistantMessage)
	if err != nil {
		return AskResult{}, err
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	s.activeMu.Lock()
	s.activeRuns[run.ID] = cancelRun
	s.activeMu.Unlock()
	defer func() { cancelRun(); s.activeMu.Lock(); delete(s.activeRuns, run.ID); s.activeMu.Unlock() }()
	if conversation.Title == "" || conversation.Title == "新对话" {
		conversation.Title = truncateRunes(userMessage.Content, 40)
		conversation.UpdatedAt = now
		_, _ = s.repository.UpdateConversation(ctx, userID, conversation)
	}
	events := make([]StreamEvent, 0, 12)
	emit := func(eventType string, payload map[string]any) {
		event := StreamEvent{EventSeq: len(events) + 1, EventType: eventType, Payload: payload, CreatedAt: s.now().UTC()}
		events = append(events, event)
		emitProgress(observe, ProgressEvent{Type: eventType, Sequence: event.EventSeq, Payload: payload, UserMessageID: userMessage.ID, AssistantMessage: assistantMessage.ID, Intent: intent})
	}
	emit("message.created", map[string]any{"responseRunId": run.ID, "userMessageId": userMessage.ID, "assistantMessageId": assistantMessage.ID, "status": "running"})

	runtime, release, err := s.runtime.Acquire()
	if err != nil {
		return AskResult{}, NewError(CodeDependency, "agent runtime is unavailable", err)
	}
	defer release()
	messages := make([]agent.Message, 0, len(history.Items)+3)
	messages = append(messages, agent.Message{Role: agent.RoleSystem, Content: runtime.SystemPrompt})
	if directive := requestDirective(input); directive != "" {
		messages = append(messages, agent.Message{Role: agent.RoleSystem, Content: directive})
	}
	for _, item := range history.Items {
		if item.Status == "completed" && (item.Role == agent.RoleUser || item.Role == agent.RoleAssistant) {
			messages = append(messages, agent.Message{Role: item.Role, Content: item.Content})
		}
	}
	messages = append(messages, agent.Message{Role: agent.RoleUser, Content: userMessage.Content})

	steps := make([]ReasoningStep, 0, 4)
	result, runErr := runtime.Runner.RunWithObserver(runCtx, messages, func(event agent.Event) {
		switch event.Type {
		case agent.EventModelStarted:
			emit("agent.iteration.started", map[string]any{"responseRunId": run.ID, "iterationNo": event.Iteration})
		case agent.EventToolStarted, agent.EventToolCompleted, agent.EventToolFailed:
			emit(string(event.Type), map[string]any{"toolCallId": event.ToolCallID, "tool": event.ToolName, "iterationNo": event.Iteration})
		}
		step, ok := stepFromAgentEvent(assistantMessage.ID, event, s.now().UTC())
		if !ok {
			return
		}
		steps = append(steps, step)
		emit("reasoning.step", map[string]any{"type": publicStepType(step.Type), "label": step.Title, "status": publicStepStatus(step.Status), "detail": step.Summary})
	})
	if runErr == nil && runCtx.Err() != nil {
		runErr = runCtx.Err()
	}
	if runErr != nil {
		assistantMessage.Status = "failed"
		if errors.Is(runErr, context.Canceled) {
			assistantMessage.Status = "cancelled"
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.repository.UpdateMessage(cleanupCtx, userID, assistantMessage)
		_ = s.repository.SaveReasoningSteps(cleanupCtx, userID, assistantMessage.ID, steps)
		emit("error", map[string]any{"responseRunId": run.ID, "code": "dependency_error", "message": "answer generation failed"})
		_ = s.repository.SaveStreamEvents(cleanupCtx, userID, run.ID, events)
		return AskResult{UserMessage: userMessage, AssistantMessage: assistantMessage, ResponseRun: run, Citations: []any{}, ReasoningSteps: steps}, NewError(CodeDependency, "answer generation failed", runErr)
	}
	assistantMessage.Content = result.Final.Content
	assistantMessage.Status = "completed"
	if err := s.repository.UpdateMessage(ctx, userID, assistantMessage); err != nil {
		return AskResult{}, fmt.Errorf("save assistant message: %w", err)
	}
	if err := s.repository.SaveReasoningSteps(ctx, userID, assistantMessage.ID, steps); err != nil {
		return AskResult{}, fmt.Errorf("save reasoning steps: %w", err)
	}
	emit("answer.delta", map[string]any{"messageId": assistantMessage.ID, "text": assistantMessage.Content, "index": 0})
	emit("answer.completed", map[string]any{"responseRunId": run.ID, "messageId": assistantMessage.ID})
	if err := s.repository.SaveStreamEvents(ctx, userID, run.ID, events); err != nil {
		return AskResult{}, fmt.Errorf("save stream events: %w", err)
	}
	if completed, err := s.repository.GetResponseRun(ctx, userID, run.ID); err == nil {
		run = completed
	}
	return AskResult{UserMessage: userMessage, AssistantMessage: assistantMessage, ResponseRun: run, Citations: []any{}, ReasoningSteps: steps}, nil
}

func (s *QAService) CancelActiveRun(runID string) {
	s.activeMu.Lock()
	cancel := s.activeRuns[runID]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func publicStepType(value string) string {
	if value == "tool" {
		return "tool_call"
	}
	return value
}

func publicStepStatus(value string) string {
	if value == "completed" {
		return "done"
	}
	return value
}

func validateAskInput(input AskInput) error {
	message := strings.TrimSpace(input.Message)
	if message == "" || utf8.RuneCountInString(message) > 32000 {
		return ValidationError(map[string]string{"message": "must be between 1 and 32000 characters"})
	}
	allowedModes := map[string]bool{"": true, "knowledge_qa": true, "general_chat": true, "report_generation": true, "data_analysis": true, "unknown": true}
	if !allowedModes[input.Mode] {
		return ValidationError(map[string]string{"mode": "is not supported"})
	}
	if input.Mode == "data_analysis" {
		return NewError(CodeUnsupportedIntent, "data analysis is not supported", nil)
	}
	if len(input.KnowledgeBaseIDs) > 50 {
		return ValidationError(map[string]string{"knowledgeBaseIds": "must not contain more than 50 items"})
	}
	return nil
}

func requestDirective(input AskInput) string {
	var parts []string
	if input.Mode != "" && input.Mode != "unknown" {
		parts = append(parts, "The requested QA mode is "+input.Mode+".")
	}
	if len(input.KnowledgeBaseIDs) > 0 {
		parts = append(parts, "When a knowledge tool supports knowledge-base filtering, restrict it to: "+strings.Join(input.KnowledgeBaseIDs, ", ")+".")
	}
	return strings.Join(parts, " ")
}

func stepFromAgentEvent(messageID string, event agent.Event, now time.Time) (ReasoningStep, bool) {
	step := ReasoningStep{ID: newID("step"), MessageID: messageID, Status: "completed", CreatedAt: now}
	switch event.Type {
	case agent.EventModelStarted:
		step.Type, step.Title, step.Summary, step.Status = "generation", "生成回答", "模型开始处理当前步骤", "running"
	case agent.EventModelCompleted:
		step.Type, step.Title, step.Summary = "generation", "生成回答", "模型完成当前步骤"
	case agent.EventToolStarted:
		step.Type, step.Title, step.Summary, step.Status = "tool", "调用工具", "开始调用工具 "+event.ToolName, "running"
	case agent.EventToolCompleted:
		step.Type, step.Title, step.Summary = "tool", "调用工具", "工具 "+event.ToolName+" 调用完成"
	case agent.EventToolFailed:
		step.Type, step.Title, step.Summary, step.Status = "tool", "调用工具", "工具 "+event.ToolName+" 调用失败", "failed"
	default:
		return ReasoningStep{}, false
	}
	return step, true
}

func emitProgress(observer ProgressObserver, event ProgressEvent) {
	if observer != nil {
		observer(event)
	}
}

func newID(prefix string) string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(data)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
