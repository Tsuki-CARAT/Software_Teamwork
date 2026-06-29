package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func (r *Postgres) GetActiveQAConfig(ctx context.Context) (service.RetrievalSettings, []string, error) {
	var settings service.RetrievalSettings
	var configID string
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, top_k, similarity_threshold, use_rerank,
		       COALESCE(rerank_threshold, 0.5), COALESCE(rerank_top_n, 3)
		FROM qa_config_versions WHERE is_active = true
		ORDER BY version_no DESC LIMIT 1`).Scan(
		&configID, &settings.TopK, &settings.ScoreThreshold, &settings.EnableRerank,
		&settings.RerankThreshold, &settings.RerankTopN,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.RetrievalSettings{TopK: 5, ScoreThreshold: 0.7, RerankThreshold: 0.5, RerankTopN: 3}, []string{}, nil
	}
	if err != nil {
		return service.RetrievalSettings{}, nil, fmt.Errorf("get active QA config: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT external_kb_id FROM qa_config_knowledge_bases
		WHERE config_id = $1 ORDER BY sort_order, external_kb_id`, configID)
	if err != nil {
		return service.RetrievalSettings{}, nil, fmt.Errorf("list QA config knowledge bases: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return service.RetrievalSettings{}, nil, fmt.Errorf("scan QA config knowledge base: %w", err)
		}
		ids = append(ids, id)
	}
	return settings, ids, rows.Err()
}

func (r *Postgres) CreateQAConfigVersion(ctx context.Context, userID string, settings service.RetrievalSettings, knowledgeBaseIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin QA config update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `LOCK TABLE qa_config_versions IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock QA config versions: %w", err)
	}
	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_no), 0) + 1 FROM qa_config_versions`).Scan(&version); err != nil {
		return fmt.Errorf("next QA config version: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE qa_config_versions SET is_active = false WHERE is_active = true`); err != nil {
		return fmt.Errorf("deactivate QA config: %w", err)
	}
	var configID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO qa_config_versions (
			version_no, top_k, similarity_threshold, use_rerank,
			rerank_threshold, rerank_top_n, is_active, created_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, true, $7)
		RETURNING id::text`, version, settings.TopK, settings.ScoreThreshold,
		settings.EnableRerank, settings.RerankThreshold, settings.RerankTopN, userID).Scan(&configID); err != nil {
		return fmt.Errorf("insert QA config version: %w", err)
	}
	for index, id := range knowledgeBaseIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO qa_config_knowledge_bases (config_id, external_kb_id, sort_order)
			VALUES ($1, $2, $3)`, configID, id, index); err != nil {
			return fmt.Errorf("insert QA config knowledge base: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit QA config update: %w", err)
	}
	return nil
}

func (r *Postgres) GetActiveLLMConfig(ctx context.Context) (service.StoredLLMConfig, error) {
	var config service.StoredLLMConfig
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, provider, COALESCE(profile_id, ''), COALESCE(api_endpoint, ''), api_key_encrypted,
		       COALESCE(api_key_last4, ''), token_header, model_name,
		       timeout_seconds, temperature, max_tokens
		FROM llm_config_versions WHERE is_active = true
		ORDER BY version_no DESC LIMIT 1`).Scan(
		&config.ID, &config.Provider, &config.ProfileID, &config.APIEndpoint, &config.APIKeyEncrypted,
		&config.APIKeyLast4, &config.TokenHeader, &config.Model,
		&config.TimeoutSeconds, &config.Temperature, &config.MaxTokens,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.StoredLLMConfig{}, service.NewError(service.CodeNotFound, "active LLM configuration not found", err)
	}
	if err != nil {
		return service.StoredLLMConfig{}, fmt.Errorf("get active LLM config: %w", err)
	}
	return config, nil
}

func (r *Postgres) CreateLLMConfigVersion(ctx context.Context, userID string, config service.StoredLLMConfig) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin LLM config update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `LOCK TABLE llm_config_versions IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock LLM config versions: %w", err)
	}
	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version_no), 0) + 1 FROM llm_config_versions`).Scan(&version); err != nil {
		return fmt.Errorf("next LLM config version: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE llm_config_versions SET is_active = false WHERE is_active = true`); err != nil {
		return fmt.Errorf("deactivate LLM config: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO llm_config_versions (
			version_no, provider, profile_id, api_endpoint, api_key_encrypted,
			api_key_last4, token_header, model_name, timeout_seconds,
			temperature, max_tokens, is_active, created_by_user_id
		) VALUES ($1, 'direct', NULL, $2, $3, $4, $5, $6, $7, $8, $9, true, $10)`,
		version, config.APIEndpoint, config.APIKeyEncrypted, config.APIKeyLast4,
		config.TokenHeader, config.Model, config.TimeoutSeconds, config.Temperature,
		config.MaxTokens, userID)
	if err != nil {
		return fmt.Errorf("insert LLM config version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit LLM config update: %w", err)
	}
	return nil
}

func (r *Postgres) GetRuntimeSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx, `SELECT value FROM qa_runtime_settings WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", service.NewError(service.CodeNotFound, "runtime setting not found", err)
	}
	if err != nil {
		return "", fmt.Errorf("get runtime setting: %w", err)
	}
	return value, nil
}

func (r *Postgres) UpsertRuntimeSetting(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO qa_runtime_settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	if err != nil {
		return fmt.Errorf("upsert runtime setting: %w", err)
	}
	return nil
}

func (r *Postgres) ListMCPServers(ctx context.Context) ([]service.MCPServerRecord, error) {
	rows, err := r.pool.Query(ctx, mcpServerSelect+` ORDER BY sort_order, alias`)
	if err != nil {
		return nil, fmt.Errorf("list MCP servers: %w", err)
	}
	defer rows.Close()
	servers := make([]service.MCPServerRecord, 0)
	for rows.Next() {
		server, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (r *Postgres) GetMCPServer(ctx context.Context, id string) (service.MCPServerRecord, error) {
	server, err := scanMCPServer(r.pool.QueryRow(ctx, mcpServerSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return service.MCPServerRecord{}, service.NewError(service.CodeNotFound, "MCP server not found", err)
	}
	return server, err
}

func (r *Postgres) CreateMCPServer(ctx context.Context, server service.MCPServerRecord) (service.MCPServerRecord, error) {
	argsJSON, _ := json.Marshal(server.Args)
	err := r.pool.QueryRow(ctx, `
		INSERT INTO mcp_servers (
			alias, display_name, transport, command, args_json, endpoint_url,
			token_encrypted, token_last4, token_header, tool_timeout_seconds,
			enabled, sort_order, created_by_user_id
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, NULLIF($8, ''), $9, $10, $11, $12, $13)
		RETURNING id::text, created_at, updated_at`,
		server.Alias, server.DisplayName, server.Transport, server.Command, argsJSON,
		server.EndpointURL, server.TokenEncrypted, server.TokenLast4, server.TokenHeader,
		server.ToolTimeoutSeconds, server.Enabled, server.SortOrder, server.CreatedByUserID,
	).Scan(&server.ID, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return service.MCPServerRecord{}, fmt.Errorf("insert MCP server: %w", err)
	}
	return server, nil
}

func (r *Postgres) UpdateMCPServer(ctx context.Context, server service.MCPServerRecord) (service.MCPServerRecord, error) {
	argsJSON, _ := json.Marshal(server.Args)
	err := r.pool.QueryRow(ctx, `
		UPDATE mcp_servers SET
			display_name = $1, transport = $2, command = NULLIF($3, ''),
			args_json = $4, endpoint_url = NULLIF($5, ''), token_encrypted = $6,
			token_last4 = NULLIF($7, ''), token_header = $8,
			tool_timeout_seconds = $9, enabled = $10, sort_order = $11, updated_at = now()
		WHERE id = $12
		RETURNING updated_at`, server.DisplayName, server.Transport, server.Command,
		argsJSON, server.EndpointURL, server.TokenEncrypted, server.TokenLast4,
		server.TokenHeader, server.ToolTimeoutSeconds, server.Enabled, server.SortOrder,
		server.ID).Scan(&server.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.MCPServerRecord{}, service.NewError(service.CodeNotFound, "MCP server not found", err)
	}
	if err != nil {
		return service.MCPServerRecord{}, fmt.Errorf("update MCP server: %w", err)
	}
	return server, nil
}

func (r *Postgres) DeleteMCPServer(ctx context.Context, id string) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM mcp_servers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete MCP server: %w", err)
	}
	if command.RowsAffected() == 0 {
		return service.NewError(service.CodeNotFound, "MCP server not found", nil)
	}
	return nil
}

func (r *Postgres) UpdateMCPConnectionStatus(ctx context.Context, id string, toolCount int, connectedAt *time.Time, lastError string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE mcp_servers SET tool_count = $1, last_connected_at = $2,
		       last_error = NULLIF($3, ''), updated_at = now()
		WHERE id = $4`, toolCount, connectedAt, lastError, id)
	if err != nil {
		return fmt.Errorf("update MCP connection status: %w", err)
	}
	return nil
}

func (r *Postgres) WriteAuditLog(ctx context.Context, audit service.AuditLog) error {
	beforeJSON, err := json.Marshal(audit.BeforeData)
	if err != nil {
		return fmt.Errorf("encode audit before data: %w", err)
	}
	afterJSON, err := json.Marshal(audit.AfterData)
	if err != nil {
		return fmt.Errorf("encode audit after data: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			external_user_id, action, target_type, target_id,
			before_data, after_data, request_id
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''))`,
		audit.UserID, audit.Action, audit.TargetType, audit.TargetID,
		beforeJSON, afterJSON, audit.RequestID)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

const mcpServerSelect = `
	SELECT id::text, alias, display_name, transport, COALESCE(command, ''),
	       args_json, COALESCE(endpoint_url, ''), token_encrypted,
	       COALESCE(token_last4, ''), token_header, tool_timeout_seconds,
	       enabled, sort_order, tool_count, last_connected_at,
	       COALESCE(last_error, ''), created_by_user_id, created_at, updated_at
	FROM mcp_servers`

type rowScanner interface {
	Scan(...any) error
}

func scanMCPServer(row rowScanner) (service.MCPServerRecord, error) {
	var server service.MCPServerRecord
	var argsJSON []byte
	err := row.Scan(
		&server.ID, &server.Alias, &server.DisplayName, &server.Transport,
		&server.Command, &argsJSON, &server.EndpointURL, &server.TokenEncrypted,
		&server.TokenLast4, &server.TokenHeader, &server.ToolTimeoutSeconds,
		&server.Enabled, &server.SortOrder, &server.ToolCount,
		&server.LastConnectedAt, &server.LastError, &server.CreatedByUserID,
		&server.CreatedAt, &server.UpdatedAt,
	)
	if err != nil {
		return service.MCPServerRecord{}, err
	}
	if err := json.Unmarshal(argsJSON, &server.Args); err != nil {
		return service.MCPServerRecord{}, fmt.Errorf("decode MCP server arguments: %w", err)
	}
	return server, nil
}
