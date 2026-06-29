package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("QA_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("QA_DATABASE_URL is invalid")
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
	return &Postgres{pool: pool}, nil
}

func (r *Postgres) Close() { r.pool.Close() }

func (r *Postgres) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

func (r *Postgres) CreateConversation(ctx context.Context, conversation service.Conversation) (service.Conversation, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO conversations (id, external_user_id, title, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		conversation.ID, conversation.OwnerUserID, conversation.Title, conversation.Status, conversation.CreatedAt, conversation.UpdatedAt)
	if err != nil {
		return service.Conversation{}, fmt.Errorf("insert conversation: %w", err)
	}
	return conversation, nil
}

func (r *Postgres) ListConversations(ctx context.Context, userID string, page, pageSize int, query string) (service.Page[service.Conversation], error) {
	query = strings.TrimSpace(query)
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM conversations
		WHERE external_user_id = $1 AND deleted_at IS NULL AND ($2 = '' OR title ILIKE '%' || $2 || '%')`, userID, query).Scan(&total); err != nil {
		return service.Page[service.Conversation]{}, fmt.Errorf("count conversations: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.title, c.external_user_id, c.status, c.created_at, c.updated_at, c.last_message_at,
		       (SELECT count(*) FROM messages m WHERE m.conversation_id=c.id),
		       COALESCE((SELECT left(b.content,200) FROM messages m JOIN message_content_blocks b ON b.message_id=m.id AND b.block_order=0 WHERE m.conversation_id=c.id ORDER BY m.sequence_no DESC LIMIT 1),'')
		FROM conversations c
		WHERE c.external_user_id = $1 AND c.deleted_at IS NULL AND ($2 = '' OR c.title ILIKE '%' || $2 || '%')
		ORDER BY COALESCE(c.last_message_at, c.created_at) DESC
		LIMIT $3 OFFSET $4`, userID, query, pageSize, (page-1)*pageSize)
	if err != nil {
		return service.Page[service.Conversation]{}, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	items := make([]service.Conversation, 0)
	for rows.Next() {
		conversation, err := scanConversation(rows)
		if err != nil {
			return service.Page[service.Conversation]{}, err
		}
		items = append(items, conversation)
	}
	if err := rows.Err(); err != nil {
		return service.Page[service.Conversation]{}, fmt.Errorf("iterate conversations: %w", err)
	}
	return service.Page[service.Conversation]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (r *Postgres) GetConversation(ctx context.Context, userID, id string) (service.Conversation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id::text, c.title, c.external_user_id, c.status, c.created_at, c.updated_at, c.last_message_at,
		       (SELECT count(*) FROM messages m WHERE m.conversation_id=c.id),
		       COALESCE((SELECT left(b.content,200) FROM messages m JOIN message_content_blocks b ON b.message_id=m.id AND b.block_order=0 WHERE m.conversation_id=c.id ORDER BY m.sequence_no DESC LIMIT 1),'')
		FROM conversations c WHERE c.id::text = $1 AND c.external_user_id = $2 AND c.deleted_at IS NULL`, id, userID)
	conversation, err := scanConversation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.Conversation{}, service.NewError(service.CodeNotFound, "conversation not found", err)
	}
	if err != nil {
		return service.Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	return conversation, nil
}

func (r *Postgres) UpdateConversation(ctx context.Context, userID string, conversation service.Conversation) (service.Conversation, error) {
	command, err := r.pool.Exec(ctx, `
		UPDATE conversations SET title = $1, status=$2, updated_at = $3
		WHERE id::text = $4 AND external_user_id = $5 AND deleted_at IS NULL`,
		conversation.Title, conversation.Status, conversation.UpdatedAt, conversation.ID, userID)
	if err != nil {
		return service.Conversation{}, fmt.Errorf("update conversation: %w", err)
	}
	if command.RowsAffected() == 0 {
		return service.Conversation{}, service.NewError(service.CodeNotFound, "conversation not found", nil)
	}
	return conversation, nil
}

func (r *Postgres) DeleteConversation(ctx context.Context, userID, id string) error {
	command, err := r.pool.Exec(ctx, `
		UPDATE conversations SET deleted_at = now(), updated_at = now()
		WHERE id::text = $1 AND external_user_id = $2 AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if command.RowsAffected() == 0 {
		return service.NewError(service.CodeNotFound, "conversation not found", nil)
	}
	return nil
}

func (r *Postgres) ListMessages(ctx context.Context, userID, conversationID string, page, pageSize int) (service.Page[service.Message], error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM messages m JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id::text = $1 AND c.external_user_id = $2 AND c.deleted_at IS NULL`, conversationID, userID).Scan(&total)
	if err != nil {
		return service.Page[service.Message]{}, fmt.Errorf("count messages: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text, m.conversation_id::text, m.sequence_no, m.role,
		       COALESCE(b.content, ''), COALESCE(m.intent, ''), m.status, m.created_at, m.completed_at,
		       (SELECT count(*) FROM citations ci WHERE ci.message_id = m.id)
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		LEFT JOIN message_content_blocks b ON b.message_id = m.id AND b.block_order = 0
		WHERE m.conversation_id::text = $1 AND c.external_user_id = $2 AND c.deleted_at IS NULL
		ORDER BY m.sequence_no
		LIMIT $3 OFFSET $4`, conversationID, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return service.Page[service.Message]{}, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	items := make([]service.Message, 0)
	for rows.Next() {
		var message service.Message
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.SequenceNo, &message.Role, &message.Content, &message.Intent, &message.Status, &message.CreatedAt, &message.CompletedAt, &message.CitationCount); err != nil {
			return service.Page[service.Message]{}, fmt.Errorf("scan message: %w", err)
		}
		if message.Status == "generating" {
			message.Status = "streaming"
		}
		items = append(items, message)
	}
	if err := rows.Err(); err != nil {
		return service.Page[service.Message]{}, fmt.Errorf("iterate messages: %w", err)
	}
	if total == 0 {
		if _, err := r.GetConversation(ctx, userID, conversationID); err != nil {
			return service.Page[service.Message]{}, err
		}
	}
	return service.Page[service.Message]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (r *Postgres) AppendMessages(ctx context.Context, userID, conversationID string, messages ...service.Message) (service.ResponseRun, error) {
	if len(messages) == 0 {
		return service.ResponseRun{}, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return service.ResponseRun{}, fmt.Errorf("begin append messages: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT true FROM conversations
		WHERE id::text = $1 AND external_user_id = $2 AND deleted_at IS NULL FOR UPDATE`, conversationID, userID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return service.ResponseRun{}, service.NewError(service.CodeNotFound, "conversation not found", err)
	} else if err != nil {
		return service.ResponseRun{}, fmt.Errorf("lock conversation: %w", err)
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence_no), 0) FROM messages WHERE conversation_id::text = $1`, conversationID).Scan(&sequence); err != nil {
		return service.ResponseRun{}, fmt.Errorf("get message sequence: %w", err)
	}
	var userMessageID, assistantMessageID, intent string
	for _, message := range messages {
		sequence++
		if _, err := tx.Exec(ctx, `
			INSERT INTO messages (id, conversation_id, role, sequence_no, intent, status, created_at, completed_at)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7,
			        CASE WHEN $6 = 'completed' THEN $7::timestamptz ELSE NULL END)`,
			message.ID, conversationID, message.Role, sequence, message.Intent, message.Status, message.CreatedAt); err != nil {
			return service.ResponseRun{}, fmt.Errorf("insert message: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO message_content_blocks (message_id, block_order, content, status, created_at, updated_at)
			VALUES ($1, 0, $2, $3, $4, $4)`, message.ID, message.Content, blockStatus(message.Status), message.CreatedAt); err != nil {
			return service.ResponseRun{}, fmt.Errorf("insert message content: %w", err)
		}
		if message.Role == "user" {
			userMessageID = message.ID
		}
		if message.Role == "assistant" {
			assistantMessageID, intent = message.ID, message.Intent
		}
	}
	lastAt := messages[len(messages)-1].CreatedAt
	if _, err := tx.Exec(ctx, `UPDATE conversations SET updated_at = $1, last_message_at = $1 WHERE id = $2`, lastAt, conversationID); err != nil {
		return service.ResponseRun{}, fmt.Errorf("touch conversation: %w", err)
	}
	var run service.ResponseRun
	if userMessageID != "" && assistantMessageID != "" {
		if err := tx.QueryRow(ctx, `
			INSERT INTO response_runs (conversation_id, user_message_id, assistant_message_id, intent_type, route, status)
			VALUES ($1, $2, $3, NULLIF($4, ''), 'agent', 'running')
			RETURNING id::text, conversation_id::text, user_message_id::text,
			          assistant_message_id::text, status, started_at`,
			conversationID, userMessageID, assistantMessageID, intent).Scan(
			&run.ID, &run.SessionID, &run.UserMessageID, &run.AssistantMessageID,
			&run.Status, &run.CreatedAt,
		); err != nil {
			return service.ResponseRun{}, fmt.Errorf("insert response run: %w", err)
		}
		run.MaxIterations = 5
	}
	if err := tx.Commit(ctx); err != nil {
		return service.ResponseRun{}, fmt.Errorf("commit append messages: %w", err)
	}
	return run, nil
}

func (r *Postgres) UpdateMessage(ctx context.Context, userID string, message service.Message) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update message: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
		UPDATE messages m SET status = $1, intent = NULLIF($2, ''),
		       completed_at = CASE WHEN $1 IN ('completed', 'failed', 'cancelled') THEN now() ELSE NULL END
		FROM conversations c
		WHERE m.id = $3 AND c.id = m.conversation_id AND c.external_user_id = $4 AND c.deleted_at IS NULL`,
		message.Status, message.Intent, message.ID, userID)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}
	if command.RowsAffected() == 0 {
		return service.NewError(service.CodeNotFound, "message not found", nil)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE message_content_blocks SET content = $1, status = $2, updated_at = now()
		WHERE message_id = $3 AND block_order = 0`, message.Content, blockStatus(message.Status), message.ID); err != nil {
		return fmt.Errorf("update message content: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE response_runs SET status = $1,
		       stop_reason = CASE WHEN $1 = 'completed' THEN NULL ELSE $1 END,
		       completed_at = CASE WHEN $1 <> 'running' THEN now() ELSE NULL END,
		       latency_ms = CASE WHEN $1 <> 'running' THEN EXTRACT(EPOCH FROM (now() - started_at)) * 1000 ELSE NULL END
		WHERE assistant_message_id = $2`, runStatus(message.Status), message.ID); err != nil {
		return fmt.Errorf("update response run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update message: %w", err)
	}
	return nil
}

func (r *Postgres) SaveReasoningSteps(ctx context.Context, userID, assistantMessageID string, steps []service.ReasoningStep) error {
	if len(steps) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save reasoning steps: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var runID string
	if err := tx.QueryRow(ctx, `
		SELECT rr.id::text FROM response_runs rr
		JOIN conversations c ON c.id = rr.conversation_id
		WHERE rr.assistant_message_id = $1 AND c.external_user_id = $2`, assistantMessageID, userID).Scan(&runID); errors.Is(err, pgx.ErrNoRows) {
		return service.NewError(service.CodeNotFound, "response run not found", err)
	} else if err != nil {
		return fmt.Errorf("find response run: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM response_process_steps WHERE response_run_id = $1`, runID); err != nil {
		return fmt.Errorf("replace reasoning steps: %w", err)
	}
	for index, step := range steps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO response_process_steps (id, response_run_id, step_order, step_type, label, detail, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			step.ID, runID, index+1, step.Type, step.Title, step.Summary, step.Status, step.CreatedAt); err != nil {
			return fmt.Errorf("insert reasoning step: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reasoning steps: %w", err)
	}
	return nil
}

func (r *Postgres) SaveStreamEvents(ctx context.Context, userID, runID string, events []service.StreamEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save stream events: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM response_runs rr JOIN conversations c ON c.id=rr.conversation_id WHERE rr.id::text=$1 AND c.external_user_id=$2`, runID, userID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return service.NewError(service.CodeNotFound, "response run not found", err)
	} else if err != nil {
		return fmt.Errorf("authorize stream events: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM response_stream_events WHERE response_run_id=$1`, runID); err != nil {
		return fmt.Errorf("replace stream events: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM agent_tool_calls WHERE response_run_id=$1`, runID); err != nil {
		return fmt.Errorf("replace tool call summaries: %w", err)
	}
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return fmt.Errorf("encode stream event: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO response_stream_events(response_run_id,event_seq,event_type,payload,created_at) VALUES($1,$2,$3,$4,$5)`, runID, event.EventSeq, event.EventType, payload, event.CreatedAt); err != nil {
			return fmt.Errorf("insert stream event: %w", err)
		}
		iteration, _ := event.Payload["iterationNo"].(int)
		if event.EventType == "agent.iteration.started" && iteration > 0 {
			if _, err = tx.Exec(ctx, `UPDATE response_runs SET current_iteration=GREATEST(current_iteration,$1) WHERE id=$2`, iteration, runID); err != nil {
				return fmt.Errorf("update response run iteration: %w", err)
			}
		}
		if event.EventType == "tool.started" || event.EventType == "tool.completed" || event.EventType == "tool.failed" {
			toolCallID, _ := event.Payload["toolCallId"].(string)
			toolName, _ := event.Payload["tool"].(string)
			if toolCallID == "" {
				continue
			}
			status := "running"
			if event.EventType == "tool.completed" {
				status = "completed"
			}
			if event.EventType == "tool.failed" {
				status = "failed"
			}
			if _, err = tx.Exec(ctx, `
				INSERT INTO agent_tool_calls(response_run_id,iteration_no,tool_call_id,tool_name,status,started_at,finished_at)
				VALUES($1,GREATEST($2,1),$3,$4,$5,$6::timestamptz,CASE WHEN $5='running' THEN NULL ELSE $6::timestamptz END)
				ON CONFLICT(response_run_id,tool_call_id) DO UPDATE SET
				  status=EXCLUDED.status,
				  finished_at=CASE WHEN EXCLUDED.status='running' THEN agent_tool_calls.finished_at ELSE EXCLUDED.finished_at END,
				  latency_ms=CASE WHEN EXCLUDED.status='running' THEN agent_tool_calls.latency_ms ELSE EXTRACT(EPOCH FROM(EXCLUDED.finished_at-agent_tool_calls.started_at))*1000 END`,
				runID, iteration, toolCallID, toolName, status, event.CreatedAt); err != nil {
				return fmt.Errorf("save tool call summary: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stream events: %w", err)
	}
	return nil
}

func (r *Postgres) GetResponseRun(ctx context.Context, userID, runID string) (service.ResponseRun, error) {
	return scanResponseRun(r.pool.QueryRow(ctx, responseRunSelect+` WHERE rr.id::text=$1 AND c.external_user_id=$2 AND c.deleted_at IS NULL`, runID, userID))
}

func (r *Postgres) CancelResponseRun(ctx context.Context, userID, runID string) (service.ResponseRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return service.ResponseRun{}, fmt.Errorf("begin cancel response run: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var assistantID string
	err = tx.QueryRow(ctx, `UPDATE response_runs rr SET status='cancelled',stop_reason='cancelled',completed_at=now(),latency_ms=EXTRACT(EPOCH FROM(now()-started_at))*1000 FROM conversations c WHERE rr.id::text=$1 AND c.id=rr.conversation_id AND c.external_user_id=$2 AND c.deleted_at IS NULL AND rr.status IN('running') RETURNING rr.assistant_message_id::text`, runID, userID).Scan(&assistantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ResponseRun{}, service.NewError(service.CodeConflict, "response run cannot be cancelled", err)
	}
	if err != nil {
		return service.ResponseRun{}, fmt.Errorf("cancel response run: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE messages SET status='cancelled',completed_at=now() WHERE id=$1`, assistantID); err != nil {
		return service.ResponseRun{}, fmt.Errorf("cancel assistant message: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE message_content_blocks SET status='cancelled',updated_at=now() WHERE message_id=$1`, assistantID); err != nil {
		return service.ResponseRun{}, fmt.Errorf("cancel assistant content: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return service.ResponseRun{}, fmt.Errorf("commit response run cancellation: %w", err)
	}
	return r.GetResponseRun(ctx, userID, runID)
}

const responseRunSelect = `SELECT rr.id::text,rr.conversation_id::text,rr.user_message_id::text,rr.assistant_message_id::text,rr.status,COALESCE(rr.current_iteration,0),COALESCE(rr.max_iterations,5),rr.stop_reason,COALESCE(rr.prompt_tokens,0)+COALESCE(rr.completion_tokens,0)+COALESCE(rr.reasoning_tokens,0),COALESCE(rr.latency_ms,0),rr.started_at,rr.completed_at FROM response_runs rr JOIN conversations c ON c.id=rr.conversation_id`

func scanResponseRun(row rowScanner) (service.ResponseRun, error) {
	var value service.ResponseRun
	var termination sql.NullString
	err := row.Scan(&value.ID, &value.SessionID, &value.UserMessageID, &value.AssistantMessageID, &value.Status, &value.CurrentIteration, &value.MaxIterations, &termination, &value.TotalTokens, &value.LatencyMS, &value.CreatedAt, &value.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ResponseRun{}, service.NewError(service.CodeNotFound, "response run not found", err)
	}
	if err != nil {
		return service.ResponseRun{}, fmt.Errorf("scan response run: %w", err)
	}
	if termination.Valid {
		if termination.String == "failed" {
			termination.String = "model_error"
		}
		value.TerminationReason = &termination.String
	}
	return value, nil
}

type conversationScanner interface {
	Scan(...any) error
}

func scanConversation(scanner conversationScanner) (service.Conversation, error) {
	var conversation service.Conversation
	var lastMessageAt sql.NullTime
	if err := scanner.Scan(&conversation.ID, &conversation.Title, &conversation.OwnerUserID, &conversation.Status, &conversation.CreatedAt, &conversation.UpdatedAt, &lastMessageAt, &conversation.MessageCount, &conversation.LastMessagePreview); err != nil {
		return service.Conversation{}, err
	}
	if lastMessageAt.Valid {
		conversation.LastMessageAt = &lastMessageAt.Time
	}
	return conversation, nil
}

func blockStatus(messageStatus string) string {
	switch messageStatus {
	case "generating":
		return "generating"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	default:
		return "completed"
	}
}

func runStatus(messageStatus string) string {
	switch messageStatus {
	case "generating", "queued":
		return "running"
	case "cancelled":
		return "cancelled"
	case "failed":
		return "failed"
	default:
		return "completed"
	}
}
