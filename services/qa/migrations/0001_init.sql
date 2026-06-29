CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE qa_config_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_no INTEGER NOT NULL UNIQUE,
    top_k INTEGER NOT NULL DEFAULT 5 CHECK (top_k > 0),
    similarity_threshold NUMERIC(6,5) NOT NULL DEFAULT 0.7000,
    use_rerank BOOLEAN NOT NULL DEFAULT FALSE,
    rerank_threshold NUMERIC(6,5),
    rerank_top_n INTEGER CHECK (rerank_top_n IS NULL OR rerank_top_n > 0),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_qa_config_versions_active
    ON qa_config_versions (is_active) WHERE is_active;

CREATE TABLE qa_config_knowledge_bases (
    config_id UUID NOT NULL REFERENCES qa_config_versions(id) ON DELETE CASCADE,
    external_kb_id TEXT NOT NULL,
    kb_type TEXT,
    display_name_snapshot TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (config_id, external_kb_id)
);

CREATE TABLE llm_config_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version_no INTEGER NOT NULL UNIQUE,
    provider TEXT NOT NULL DEFAULT 'ai-gateway',
    profile_id TEXT NOT NULL,
    model_name TEXT NOT NULL,
    timeout_seconds INTEGER NOT NULL DEFAULT 60 CHECK (timeout_seconds > 0),
    temperature NUMERIC(4,3) NOT NULL DEFAULT 0.700,
    max_tokens INTEGER NOT NULL DEFAULT 4096 CHECK (max_tokens > 0),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_llm_config_versions_active
    ON llm_config_versions (is_active) WHERE is_active;

CREATE TABLE admin_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT,
    before_data JSONB,
    after_data JSONB,
    request_id TEXT,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_user_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    sequence_no INTEGER NOT NULL,
    intent TEXT,
    status TEXT NOT NULL CHECK (status IN ('queued', 'generating', 'completed', 'failed', 'cancelled')),
    model_name TEXT,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (conversation_id, sequence_no)
);

CREATE TABLE response_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    user_message_id UUID NOT NULL REFERENCES messages(id),
    assistant_message_id UUID NOT NULL UNIQUE REFERENCES messages(id),
    qa_config_version_id UUID REFERENCES qa_config_versions(id),
    llm_config_version_id UUID REFERENCES llm_config_versions(id),
    request_id TEXT,
    intent_type TEXT,
    route TEXT NOT NULL DEFAULT 'agent',
    confidence NUMERIC(6,5),
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'cancelled', 'failed')),
    stop_reason TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    reasoning_tokens INTEGER,
    latency_ms BIGINT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE message_content_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    block_order INTEGER NOT NULL,
    block_type TEXT NOT NULL DEFAULT 'text',
    content TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('generating', 'completed', 'cancelled', 'failed')),
    provider_block_id TEXT,
    provider_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id, block_order)
);

CREATE TABLE response_process_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    response_run_id UUID NOT NULL REFERENCES response_runs(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL,
    step_type TEXT NOT NULL,
    label TEXT NOT NULL,
    detail TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (response_run_id, step_order)
);

CREATE TABLE response_stream_events (
    id BIGSERIAL PRIMARY KEY,
    response_run_id UUID NOT NULL REFERENCES response_runs(id) ON DELETE CASCADE,
    event_seq INTEGER NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('intent', 'step', 'token', 'citation', 'done', 'error')),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours'),
    UNIQUE (response_run_id, event_seq)
);

CREATE TABLE citations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    citation_no INTEGER NOT NULL,
    char_start INTEGER,
    char_end INTEGER,
    external_kb_id TEXT,
    external_doc_id TEXT,
    external_chunk_id TEXT,
    doc_name TEXT NOT NULL,
    section_path TEXT,
    quote_text TEXT,
    context TEXT,
    page_number INTEGER,
    score NUMERIC(8,7),
    rerank_score NUMERIC(8,7),
    chunk_type TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (message_id, citation_no)
);

CREATE TABLE retrieval_test_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qa_config_version_id UUID REFERENCES qa_config_versions(id),
    external_user_id TEXT NOT NULL,
    query TEXT NOT NULL,
    overrides JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    result_count INTEGER,
    latency_ms BIGINT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE retrieval_test_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_run_id UUID NOT NULL REFERENCES retrieval_test_runs(id) ON DELETE CASCADE,
    rank_no INTEGER NOT NULL,
    external_kb_id TEXT,
    external_doc_id TEXT,
    external_chunk_id TEXT,
    doc_name TEXT,
    text_snapshot TEXT,
    vector_score NUMERIC(8,7),
    rerank_score NUMERIC(8,7),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (test_run_id, rank_no)
);

CREATE INDEX idx_conversations_external_user_id ON conversations(external_user_id);
CREATE INDEX idx_conversations_created_at ON conversations(created_at DESC);
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id, sequence_no);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_response_runs_conversation_id ON response_runs(conversation_id);
CREATE INDEX idx_response_runs_user_message_id ON response_runs(user_message_id);
CREATE INDEX idx_response_runs_started_at ON response_runs(started_at DESC);
CREATE INDEX idx_response_runs_request_id ON response_runs(request_id);
CREATE INDEX idx_message_content_blocks_message_id ON message_content_blocks(message_id);
CREATE INDEX idx_response_process_steps_run_id ON response_process_steps(response_run_id, step_order);
CREATE INDEX idx_response_stream_events_run_id ON response_stream_events(response_run_id, event_seq);
CREATE INDEX idx_response_stream_events_expires_at ON response_stream_events(expires_at);
CREATE INDEX idx_citations_message_id ON citations(message_id, citation_no);
CREATE INDEX idx_retrieval_test_runs_created_at ON retrieval_test_runs(created_at DESC);
CREATE INDEX idx_retrieval_test_results_run_id ON retrieval_test_results(test_run_id, rank_no);
CREATE INDEX idx_admin_audit_logs_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX idx_admin_audit_logs_external_user_id ON admin_audit_logs(external_user_id);

INSERT INTO qa_config_versions (
    version_no, top_k, similarity_threshold, use_rerank, is_active, created_by_user_id
) VALUES (1, 5, 0.7000, FALSE, TRUE, 'system');

INSERT INTO llm_config_versions (
    version_no, provider, profile_id, model_name, is_active, created_by_user_id
) VALUES (1, 'ai-gateway', 'default', 'deepseek-chat', TRUE, 'system');
