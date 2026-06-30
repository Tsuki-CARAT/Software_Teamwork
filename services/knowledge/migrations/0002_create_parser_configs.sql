-- +goose Up
CREATE TABLE parser_configs (
  id text PRIMARY KEY,
  name text NOT NULL,
  backend text NOT NULL CHECK (backend IN ('builtin', 'tika', 'unstructured', 'local_ocr', 'remote_compatible')),
  enabled boolean NOT NULL DEFAULT true,
  is_default boolean NOT NULL DEFAULT false,
  concurrency integer NOT NULL CHECK (concurrency BETWEEN 1 AND 128),
  supported_content_types text[] NOT NULL DEFAULT '{}',
  endpoint_url text,
  default_parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  deleted_at timestamptz,
  CONSTRAINT parser_configs_default_enabled CHECK (NOT is_default OR enabled)
);

CREATE UNIQUE INDEX uq_parser_configs_live_name ON parser_configs (lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_parser_configs_single_default ON parser_configs (is_default) WHERE is_default AND enabled AND deleted_at IS NULL;
CREATE INDEX idx_parser_configs_enabled ON parser_configs (enabled, created_at DESC) WHERE deleted_at IS NULL;

INSERT INTO parser_configs (
  id,
  name,
  backend,
  enabled,
  is_default,
  concurrency,
  supported_content_types,
  endpoint_url,
  default_parameters,
  created_at,
  updated_at
) VALUES (
  'parser_config_builtin_default',
  'Default builtin parser',
  'builtin',
  true,
  true,
  4,
  ARRAY[
    'application/pdf',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'text/markdown',
    'text/plain'
  ],
  NULL,
  '{}'::jsonb,
  now(),
  now()
);

CREATE TABLE parser_config_audits (
  id text PRIMARY KEY,
  parser_config_id text NOT NULL REFERENCES parser_configs(id),
  actor_user_id text NOT NULL,
  action text NOT NULL,
  summary jsonb NOT NULL,
  created_at timestamptz NOT NULL
);

CREATE INDEX idx_parser_config_audits_config_created_at ON parser_config_audits (parser_config_id, created_at DESC);

ALTER TABLE processing_jobs ADD COLUMN parser_config_id text REFERENCES parser_configs(id);
ALTER TABLE processing_jobs ADD COLUMN parser_config_snapshot jsonb;

-- +goose Down
ALTER TABLE processing_jobs DROP COLUMN IF EXISTS parser_config_snapshot;
ALTER TABLE processing_jobs DROP COLUMN IF EXISTS parser_config_id;
DROP TABLE IF EXISTS parser_config_audits;
DROP TABLE IF EXISTS parser_configs;
