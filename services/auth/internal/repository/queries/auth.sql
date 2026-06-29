-- name: GetUserByID :one
SELECT
    id,
    username,
    display_name,
    email,
    phone,
    status,
    locked_until,
    last_login_at,
    created_at,
    updated_at,
    deleted_at
FROM auth_users
WHERE id = $1
    AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT
    id,
    username,
    display_name,
    email,
    phone,
    status,
    locked_until,
    last_login_at,
    created_at,
    updated_at,
    deleted_at
FROM auth_users
WHERE username = $1
    AND deleted_at IS NULL;

-- name: GetCredentialByUserID :one
SELECT
    id,
    user_id,
    credential_type,
    password_hash,
    password_hash_alg,
    password_hash_params_version,
    password_hash_params_json,
    password_changed_at,
    password_expires_at,
    failed_attempt_count,
    last_failed_at,
    created_at,
    updated_at
FROM auth_credentials
WHERE user_id = $1
    AND credential_type = $2;

-- name: ListRoleCodesByUserID :many
SELECT
    r.code
FROM user_roles ur
INNER JOIN auth_roles r
    ON r.id = ur.role_id
WHERE ur.user_id = $1
    AND r.enabled = TRUE
    AND (ur.expires_at IS NULL OR ur.expires_at > now())
ORDER BY r.code ASC;

-- name: ListPermissionCodesByUserID :many
SELECT DISTINCT
    p.code
FROM user_roles ur
INNER JOIN auth_roles r
    ON r.id = ur.role_id
INNER JOIN role_permissions rp
    ON rp.role_id = r.id
INNER JOIN auth_permissions p
    ON p.id = rp.permission_id
WHERE ur.user_id = $1
    AND r.enabled = TRUE
    AND p.enabled = TRUE
    AND (ur.expires_at IS NULL OR ur.expires_at > now())
ORDER BY p.code ASC;

-- name: GetSessionByID :one
SELECT
    s.id,
    s.user_id,
    s.access_token_hash,
    s.access_token_hash_alg,
    s.access_token_hash_key_version,
    s.token_type,
    s.status,
    s.issued_at,
    s.expires_at,
    s.last_seen_at,
    s.revoked_at,
    s.revoke_reason,
    s.client_ip,
    s.user_agent,
    s.created_request_id,
    s.revoked_request_id,
    s.created_at,
    s.updated_at
FROM auth_sessions s
WHERE s.id = $1;

-- name: GetActiveSessionByTokenHash :one
SELECT
    s.id,
    s.user_id,
    s.access_token_hash,
    s.access_token_hash_alg,
    s.access_token_hash_key_version,
    s.token_type,
    s.status,
    s.issued_at,
    s.expires_at,
    s.last_seen_at,
    s.revoked_at,
    s.revoke_reason,
    s.client_ip,
    s.user_agent,
    s.created_request_id,
    s.revoked_request_id,
    s.created_at,
    s.updated_at
FROM auth_sessions s
WHERE s.access_token_hash = $1
    AND s.status = 'active'
    AND s.expires_at > now();

-- name: CreateUser :one
INSERT INTO auth_users (
    id,
    username,
    display_name,
    email,
    phone,
    status,
    locked_until,
    last_login_at,
    created_at,
    updated_at,
    deleted_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULL)
RETURNING
    id,
    username,
    display_name,
    email,
    phone,
    status,
    locked_until,
    last_login_at,
    created_at,
    updated_at,
    deleted_at;

-- name: CreateCredential :one
INSERT INTO auth_credentials (
    id,
    user_id,
    credential_type,
    password_hash,
    password_hash_alg,
    password_hash_params_version,
    password_hash_params_json,
    password_changed_at,
    password_expires_at,
    failed_attempt_count,
    last_failed_at,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, 0, NULL, $10, $11)
RETURNING
    id,
    user_id,
    credential_type,
    password_hash,
    password_hash_alg,
    password_hash_params_version,
    password_hash_params_json,
    password_changed_at,
    password_expires_at,
    failed_attempt_count,
    last_failed_at,
    created_at,
    updated_at;

-- name: CreateSession :one
INSERT INTO auth_sessions (
    id,
    user_id,
    access_token_hash,
    access_token_hash_alg,
    access_token_hash_key_version,
    token_type,
    status,
    issued_at,
    expires_at,
    last_seen_at,
    revoked_at,
    revoke_reason,
    client_ip,
    user_agent,
    created_request_id,
    revoked_request_id,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, 'Bearer', 'active', $6, $7, NULL, NULL, NULL, $8, $9, $10, NULL, $11, $12)
RETURNING
    id,
    user_id,
    access_token_hash,
    access_token_hash_alg,
    access_token_hash_key_version,
    token_type,
    status,
    issued_at,
    expires_at,
    last_seen_at,
    revoked_at,
    revoke_reason,
    client_ip,
    user_agent,
    created_request_id,
    revoked_request_id,
    created_at,
    updated_at;

-- name: RevokeSession :one
UPDATE auth_sessions
SET status = 'revoked',
    revoked_at = $2,
    revoke_reason = $3,
    revoked_request_id = $4,
    updated_at = $2
WHERE id = $1
    AND status = 'active'
RETURNING
    id,
    user_id,
    access_token_hash,
    access_token_hash_alg,
    access_token_hash_key_version,
    token_type,
    status,
    issued_at,
    expires_at,
    last_seen_at,
    revoked_at,
    revoke_reason,
    client_ip,
    user_agent,
    created_request_id,
    revoked_request_id,
    created_at,
    updated_at;
