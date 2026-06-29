ALTER TABLE llm_config_versions
    ALTER COLUMN profile_id DROP NOT NULL,
    ADD COLUMN api_endpoint TEXT,
    ADD COLUMN api_key_encrypted BYTEA,
    ADD COLUMN api_key_last4 TEXT,
    ADD COLUMN token_header TEXT NOT NULL DEFAULT 'Authorization';

CREATE TABLE qa_runtime_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO qa_runtime_settings (key, value)
VALUES (
    'system_prompt',
    'You are a helpful QA agent. Use available tools when they are needed, and answer from tool results without inventing sources.'
)
ON CONFLICT (key) DO NOTHING;

CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias TEXT NOT NULL UNIQUE CHECK (alias ~ '^[a-z0-9_]{2,32}$'),
    display_name TEXT NOT NULL DEFAULT '',
    transport TEXT NOT NULL CHECK (transport IN ('stdio', 'streamable_http')),
    command TEXT,
    args_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(args_json) = 'array'),
    endpoint_url TEXT,
    token_encrypted BYTEA,
    token_last4 TEXT,
    token_header TEXT NOT NULL DEFAULT 'Authorization',
    tool_timeout_seconds INTEGER NOT NULL DEFAULT 30 CHECK (tool_timeout_seconds > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    tool_count INTEGER NOT NULL DEFAULT 0 CHECK (tool_count >= 0),
    last_connected_at TIMESTAMPTZ,
    last_error TEXT,
    created_by_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (transport = 'stdio' AND command IS NOT NULL AND btrim(command) <> '' AND endpoint_url IS NULL)
        OR
        (transport = 'streamable_http' AND endpoint_url IS NOT NULL AND btrim(endpoint_url) <> '' AND command IS NULL)
    )
);

CREATE INDEX idx_mcp_servers_enabled ON mcp_servers(enabled, sort_order);
