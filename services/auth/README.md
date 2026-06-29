# Auth Service

`services/auth` owns users, credentials, roles, permissions, sessions, token
hash metadata, revocations, and security events. Public frontend access still
goes through gateway; this service exposes only operational endpoints in the
current baseline.

## Current Scope

Implemented now:

- Independent Go module.
- `GET /healthz`
- `GET /readyz`
- Validated runtime configuration.
- PostgreSQL migration for auth users, credentials, roles, permissions, user
  roles, role permissions, sessions, revocations, and security events.
- Service-local `sqlc.yaml` plus explicit-column query files.
- `pgx` repository adapter for basic user, credential, session, role, and
  permission lookups.
- Repository, config, and HTTP tests.

Out of scope for this baseline:

- Registration, login, logout, password hashing, token generation, and gateway
  Redis session caching.
- Public `/api/v1/**` routes.

## Local Run

```bash
cd services/auth
go test ./...
go build ./cmd/server
AUTH_HTTP_ADDR=:8001 go run ./cmd/server
```

Check the service:

```bash
curl http://localhost:8001/healthz
curl http://localhost:8001/readyz
```

Without `AUTH_DATABASE_URL`, `/readyz` returns `503` with `postgres` marked
`not_configured`. This is intentional so the process can start locally while
readiness still reflects durable store availability.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `AUTH_HTTP_ADDR` | `:8001` | HTTP listen address. |
| `AUTH_SERVICE_VERSION` | `0.1.0` | Service version returned by health checks. |
| `AUTH_ENV` | `local` | Runtime environment label. |
| `AUTH_DATABASE_URL` | unset | PostgreSQL connection string. Required for readiness. |
| `AUTH_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |
| `AUTH_READINESS_TIMEOUT` | `2s` | PostgreSQL readiness check timeout. |

Do not log `AUTH_DATABASE_URL` because it may contain credentials.

## Migration

Install or run goose with the version selected by the project, then apply the
service-owned migrations:

```bash
cd services/auth
goose -dir migrations postgres "$AUTH_DATABASE_URL" up
```

The first migration is forward and down capable:

```bash
goose -dir migrations postgres "$AUTH_DATABASE_URL" down
```

## sqlc

The service keeps SQL query files under `internal/repository/queries/` and
`sqlc.yaml` at the service root. Generate code with the pinned command:

```bash
cd services/auth
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Generated code is committed under `internal/repository/sqlc/`; the repository
adapter maps generated rows to auth service-domain structs before returning to
service callers.

## Repository Notes

- PostgreSQL access uses `pgx/v4`.
- Query files must not use `SELECT *`.
- Repository methods return service-domain structs, not generated SQL row
  structs.
- Token hashes and password hashes must never be returned by HTTP handlers or
  written to logs.
