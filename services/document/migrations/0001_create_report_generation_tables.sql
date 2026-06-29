-- +goose Up
CREATE TABLE report_types (
    code text PRIMARY KEY,
    name text NOT NULL,
    description text,
    enabled boolean NOT NULL DEFAULT true,
    default_template_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_templates (
    id uuid PRIMARY KEY,
    template_name text NOT NULL,
    report_type text NOT NULL REFERENCES report_types(code),
    version integer NOT NULL DEFAULT 1,
    file_ref text,
    filename text NOT NULL,
    file_size bigint NOT NULL DEFAULT 0,
    structure_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    style_config_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    description text,
    enabled boolean NOT NULL DEFAULT true,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE report_materials (
    id uuid PRIMARY KEY,
    material_name text NOT NULL,
    material_type text NOT NULL,
    category text,
    file_ref text,
    filename text NOT NULL,
    file_size bigint NOT NULL DEFAULT 0,
    description text,
    tags_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    enabled boolean NOT NULL DEFAULT true,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE report_template_materials (
    id uuid PRIMARY KEY,
    template_id uuid NOT NULL REFERENCES report_templates(id),
    material_id uuid NOT NULL REFERENCES report_materials(id),
    usage_type text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (template_id, material_id, usage_type)
);

CREATE TABLE reports (
    id uuid PRIMARY KEY,
    report_name text NOT NULL,
    report_type text NOT NULL REFERENCES report_types(code),
    template_id uuid REFERENCES report_templates(id),
    topic text NOT NULL,
    specialty text,
    plant_or_business_object text,
    report_year integer,
    status text NOT NULL,
    extra_context_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    creator_id text,
    creator_name text,
    source text NOT NULL DEFAULT 'backend',
    latest_job_id uuid,
    latest_report_file_id uuid,
    generated_at timestamptz,
    exported_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT reports_status_check CHECK (status IN (
        'draft',
        'outline_generating',
        'outline_generated',
        'content_generating',
        'generated',
        'exporting',
        'exported',
        'failed',
        'deleted'
    ))
);

CREATE TABLE report_outlines (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports(id),
    outline_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    version integer NOT NULL,
    source text NOT NULL DEFAULT 'manual',
    source_job_id uuid,
    is_current boolean NOT NULL DEFAULT false,
    manual_edited boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (report_id, version)
);

CREATE TABLE report_sections (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports(id),
    outline_id uuid REFERENCES report_outlines(id),
    parent_id uuid REFERENCES report_sections(id),
    outline_node_id text,
    section_path text NOT NULL,
    title text NOT NULL,
    level integer NOT NULL,
    sort_order integer NOT NULL,
    numbering text,
    section_type text NOT NULL DEFAULT 'text',
    content text NOT NULL DEFAULT '',
    tables_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    images_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    generation_status text NOT NULL DEFAULT 'pending',
    content_source text NOT NULL DEFAULT 'manual',
    manual_edited boolean NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1,
    last_job_id uuid,
    generated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (report_id, section_path)
);

CREATE TABLE report_section_versions (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports(id),
    section_id uuid NOT NULL REFERENCES report_sections(id),
    version integer NOT NULL,
    source text NOT NULL,
    content text NOT NULL DEFAULT '',
    tables_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    job_id uuid,
    requirements text,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (section_id, version)
);

CREATE TABLE report_jobs (
    id uuid PRIMARY KEY,
    request_id text,
    source text NOT NULL DEFAULT 'api',
    job_type text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    asynq_task_id text,
    queue_name text NOT NULL DEFAULT 'document',
    report_id uuid NOT NULL REFERENCES reports(id),
    template_id uuid REFERENCES report_templates(id),
    request_payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    response_payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_snapshot_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL,
    progress_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_code text,
    error_message text,
    retry_count integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT report_jobs_status_check CHECK (status IN (
        'pending',
        'running',
        'succeeded',
        'partial_succeeded',
        'failed',
        'canceled'
    )),
    CONSTRAINT report_jobs_type_check CHECK (job_type IN (
        'outline_generation',
        'outline_regeneration',
        'content_generation',
        'content_regeneration',
        'section_regeneration',
        'report_file_creation'
    )),
    CONSTRAINT report_jobs_attempts_check CHECK (max_attempts >= 1 AND retry_count >= 0)
);

CREATE TABLE report_job_attempts (
    id uuid PRIMARY KEY,
    job_id uuid NOT NULL REFERENCES report_jobs(id),
    attempt_number integer NOT NULL,
    asynq_task_id text,
    trigger_source text NOT NULL DEFAULT 'system',
    reason text,
    request_payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL,
    error_code text,
    error_message text,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_id, attempt_number),
    CONSTRAINT report_job_attempts_status_check CHECK (status IN (
        'pending',
        'running',
        'succeeded',
        'partial_succeeded',
        'failed',
        'canceled'
    ))
);

CREATE TABLE report_events (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports(id),
    job_id uuid REFERENCES report_jobs(id),
    event_type text NOT NULL,
    message text,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_files (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL REFERENCES reports(id),
    job_id uuid REFERENCES report_jobs(id),
    filename text NOT NULL,
    file_type text NOT NULL DEFAULT 'docx',
    file_ref text,
    file_size bigint NOT NULL DEFAULT 0,
    file_status text NOT NULL DEFAULT 'pending',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT report_files_status_check CHECK (file_status IN ('pending', 'running', 'succeeded', 'failed'))
);

CREATE TABLE report_operation_logs (
    id uuid PRIMARY KEY,
    operator_id text,
    operator_name text,
    operation_type text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    request_id text,
    request_source text,
    tool_name text,
    parameter_summary_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    operation_result text NOT NULL,
    error_message text,
    metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_reports_type_status_created_at ON reports(report_type, status, created_at DESC);
CREATE INDEX idx_reports_creator_created_at ON reports(creator_id, created_at DESC);
CREATE INDEX idx_report_outlines_report_current ON report_outlines(report_id, is_current);
CREATE INDEX idx_report_sections_report_outline ON report_sections(report_id, outline_id, sort_order);
CREATE INDEX idx_report_section_versions_section ON report_section_versions(section_id, version DESC);
CREATE INDEX idx_report_jobs_report_status ON report_jobs(report_id, status, created_at DESC);
CREATE INDEX idx_report_job_attempts_job ON report_job_attempts(job_id, attempt_number);
CREATE INDEX idx_report_events_report_created_at ON report_events(report_id, created_at DESC);
CREATE INDEX idx_report_files_report_created_at ON report_files(report_id, created_at DESC);
CREATE INDEX idx_report_operation_logs_target ON report_operation_logs(target_type, target_id, created_at DESC);

INSERT INTO report_types (code, name, description, enabled)
VALUES
    ('summer_peak_inspection', '迎峰度夏检查报告', '迎峰度夏检查报告', true),
    ('coal_inventory_audit', '煤库存审计报告', '煤库存审计报告', true)
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS report_operation_logs;
DROP TABLE IF EXISTS report_files;
DROP TABLE IF EXISTS report_events;
DROP TABLE IF EXISTS report_job_attempts;
DROP TABLE IF EXISTS report_jobs;
DROP TABLE IF EXISTS report_section_versions;
DROP TABLE IF EXISTS report_sections;
DROP TABLE IF EXISTS report_outlines;
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS report_template_materials;
DROP TABLE IF EXISTS report_materials;
DROP TABLE IF EXISTS report_templates;
DROP TABLE IF EXISTS report_types;
