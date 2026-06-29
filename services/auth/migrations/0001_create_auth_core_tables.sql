-- +goose Up
CREATE TABLE IF NOT EXISTS auth_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    email TEXT,
    phone TEXT,
    status TEXT NOT NULL,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT auth_users_status_check CHECK (status IN ('active', 'disabled', 'locked'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_auth_users_username
    ON auth_users (username)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_users_status
    ON auth_users (status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_users_deleted_at
    ON auth_users (deleted_at);

CREATE TABLE IF NOT EXISTS auth_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES auth_users(id),
    credential_type TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    password_hash_alg TEXT NOT NULL,
    password_hash_params_version TEXT NOT NULL,
    password_hash_params_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    password_changed_at TIMESTAMPTZ NOT NULL,
    password_expires_at TIMESTAMPTZ,
    failed_attempt_count INTEGER NOT NULL DEFAULT 0,
    last_failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT auth_credentials_type_check CHECK (credential_type IN ('password'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_auth_credentials_user_type
    ON auth_credentials (user_id, credential_type);

CREATE TABLE IF NOT EXISTS auth_roles (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    system_role BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_auth_roles_code
    ON auth_roles (code);

CREATE INDEX IF NOT EXISTS idx_auth_roles_enabled
    ON auth_roles (enabled);

CREATE TABLE IF NOT EXISTS auth_permissions (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    domain TEXT NOT NULL,
    action TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_auth_permissions_code
    ON auth_permissions (code);

CREATE INDEX IF NOT EXISTS idx_auth_permissions_domain_action
    ON auth_permissions (domain, action);

CREATE TABLE IF NOT EXISTS user_roles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES auth_users(id),
    role_id TEXT NOT NULL REFERENCES auth_roles(id),
    assigned_by TEXT,
    assigned_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_user_roles_user_role
    ON user_roles (user_id, role_id);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id
    ON user_roles (user_id);

CREATE INDEX IF NOT EXISTS idx_user_roles_role_id
    ON user_roles (role_id);

CREATE TABLE IF NOT EXISTS role_permissions (
    id TEXT PRIMARY KEY,
    role_id TEXT NOT NULL REFERENCES auth_roles(id),
    permission_id TEXT NOT NULL REFERENCES auth_permissions(id),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_role_permissions_role_permission
    ON role_permissions (role_id, permission_id);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id
    ON role_permissions (role_id);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES auth_users(id),
    access_token_hash TEXT NOT NULL,
    access_token_hash_alg TEXT NOT NULL,
    access_token_hash_key_version TEXT NOT NULL,
    token_type TEXT NOT NULL DEFAULT 'Bearer',
    status TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    client_ip TEXT,
    user_agent TEXT,
    created_request_id TEXT,
    revoked_request_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT auth_sessions_status_check CHECK (status IN ('active', 'expired', 'revoked')),
    CONSTRAINT auth_sessions_token_type_check CHECK (token_type = 'Bearer')
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_auth_sessions_access_token_hash
    ON auth_sessions (access_token_hash);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_status
    ON auth_sessions (user_id, status);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at
    ON auth_sessions (expires_at);

CREATE TABLE IF NOT EXISTS session_revocations (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES auth_sessions(id),
    user_id TEXT NOT NULL REFERENCES auth_users(id),
    reason TEXT NOT NULL,
    revoked_by TEXT,
    request_id TEXT,
    revoked_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_session_revocations_session_id
    ON session_revocations (session_id);

CREATE INDEX IF NOT EXISTS idx_session_revocations_user_id
    ON session_revocations (user_id);

CREATE TABLE IF NOT EXISTS auth_security_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    user_id TEXT REFERENCES auth_users(id),
    session_id TEXT REFERENCES auth_sessions(id),
    username_snapshot TEXT,
    request_id TEXT,
    client_ip TEXT,
    user_agent TEXT,
    caller_service TEXT,
    status TEXT NOT NULL,
    reason_code TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_security_events_user_created_at
    ON auth_security_events (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_auth_security_events_request_id
    ON auth_security_events (request_id);

CREATE INDEX IF NOT EXISTS idx_auth_security_events_session_id
    ON auth_security_events (session_id);

-- +goose Down
DROP TABLE IF EXISTS auth_security_events;
DROP TABLE IF EXISTS session_revocations;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS auth_permissions;
DROP TABLE IF EXISTS auth_roles;
DROP TABLE IF EXISTS auth_credentials;
DROP TABLE IF EXISTS auth_users;
