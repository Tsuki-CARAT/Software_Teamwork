ALTER TABLE qa_config_versions
    ADD COLUMN max_iterations INTEGER NOT NULL DEFAULT 5 CHECK (max_iterations > 0),
    ADD COLUMN tool_timeout_seconds INTEGER NOT NULL DEFAULT 10 CHECK (tool_timeout_seconds > 0),
    ADD COLUMN model_timeout_seconds INTEGER NOT NULL DEFAULT 60 CHECK (model_timeout_seconds > 0),
    ADD COLUMN overall_timeout_seconds INTEGER NOT NULL DEFAULT 120 CHECK (overall_timeout_seconds > 0),
    ADD COLUMN enabled_tool_names JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(enabled_tool_names) = 'array');

ALTER TABLE response_runs
    ADD COLUMN current_iteration INTEGER NOT NULL DEFAULT 0 CHECK (current_iteration >= 0),
    ADD COLUMN max_iterations INTEGER NOT NULL DEFAULT 5 CHECK (max_iterations > 0);

ALTER TABLE response_stream_events
    DROP CONSTRAINT response_stream_events_event_type_check;

ALTER TABLE response_stream_events
    ADD CONSTRAINT response_stream_events_event_type_check CHECK (
        event_type IN (
            'message.created', 'agent.iteration.started', 'reasoning.step',
            'tool.started', 'tool.completed', 'tool.failed', 'answer.delta',
            'citation.delta', 'answer.completed', 'error'
        )
    );

CREATE TABLE agent_tool_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    response_run_id UUID NOT NULL REFERENCES response_runs(id) ON DELETE CASCADE,
    model_invocation_id UUID,
    iteration_no INTEGER NOT NULL DEFAULT 1 CHECK (iteration_no > 0),
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    arguments_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    latency_ms BIGINT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    UNIQUE (response_run_id, tool_call_id)
);

CREATE INDEX idx_agent_tool_calls_response_run_id
    ON agent_tool_calls(response_run_id, started_at);

CREATE TABLE llm_connection_tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_user_id TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    model_name TEXT NOT NULL,
    error_code TEXT,
    error_message TEXT,
    tested_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_llm_connection_tests_tested_at
    ON llm_connection_tests(tested_at DESC);
