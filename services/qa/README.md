# QA Agent Loop

This module is an executable QA microservice backed by PostgreSQL. It exposes
the conversation and answer endpoints from the current QA API contract, runs a
Go ReAct loop with OpenAI-compatible Function Calling, and optionally acts as
an MCP client.

## Flow

```text
user message
  -> local Function Calling tools + optional MCP initialize/tools/list
  -> all tool schemas merged into model function tools
  -> AI Gateway chat completion
  -> tool_calls? -> local handler or MCP tools/call -> role=tool message -> repeat
  -> final assistant message
```

The official MCP Go SDK owns JSON-RPC framing and lifecycle operations,
including `initialize` and `notifications/initialized`. The QA service owns the
model/tool adaptation loop, timeouts, iteration limit, result truncation, and
safe progress events.

## Built-in Function Calling tools

These tools are available without an MCP Server:

| Tool         | Behavior                                                          |
| ------------ | ----------------------------------------------------------------- |
| `read_file`  | Reads UTF-8 text under `AGENT_WORKDIR`, with optional line limit. |
| `write_file` | Writes a bounded UTF-8 file under `AGENT_WORKDIR`.                |
| `edit_file`  | Replaces the first exact text occurrence under `AGENT_WORKDIR`.   |
| `bash`       | Runs a bounded workspace command only when explicitly enabled.    |

Paths must be relative and are checked after symlink resolution so file tools
cannot intentionally leave the configured workspace. Set
`AGENT_ENABLE_COMMAND_TOOL=true` only in a trusted development environment;
the command tool is disabled by default.

## Configuration

Copy `.env.example` values into your process environment. The service does not
load `.env` files and never stores tokens in source code.

AI Gateway variables:

| Variable            | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `AI_GATEWAY_URL` | AI Gateway chat completions endpoint; defaults to `http://localhost:8086/internal/v1/chat/completions`. |
| `AI_GATEWAY_TOKEN` | Internal service token for AI Gateway. When empty, QA reuses `INTERNAL_SERVICE_TOKEN`. |
| `AI_GATEWAY_TOKEN_HEADER` | Credential header; defaults to `X-Service-Token`. |
| `AI_GATEWAY_PROFILE_ID` | Optional explicit AI Gateway chat profile ID; required by the opt-in smoke. |
| `AI_GATEWAY_TIMEOUT` | Model request timeout as a Go duration; defaults to `60s`. |
| `AI_GATEWAY_STREAM` | Optional `true` to request AI Gateway `text/event-stream` completions; defaults to non-streaming JSON. |
| `MODEL_ID` | Chat model name sent in the OpenAI-compatible request; defaults to `deepseek-chat`. |

QA does not store provider API keys or provider base URLs. Runtime provider
credentials belong to AI Gateway model profiles, and QA only sends `profile_id`,
model name, timeout, and generation parameters.

## QA -> AI Gateway smoke

`TestAIGatewaySmoke` is an opt-in cross-service check that uses the same QA
model client as the server and agent. Ordinary `go test ./...` runs skip it, so
CI does not require an external model service. When enabled, the smoke performs
one minimal chat completion, then verifies that an invalid service token and a
missing profile are returned as sanitized QA dependency errors.

Prerequisites:

1. Start AI Gateway and its PostgreSQL database.
2. Create or seed an enabled chat profile with an active credential. The
   profile may point to a controlled OpenAI-compatible provider or to a real
   provider that was explicitly configured for manual smoke testing.
3. Ensure the QA service token is represented in
   `AI_GATEWAY_SERVICE_TOKEN_HASHES` and `MODEL_ID` exactly matches the selected
   profile model. See the
   [AI Gateway seed runbook](../../docs/services/ai-gateway/docs/seed-runbook.md).

PowerShell:

```powershell
cd D:\PROJECTS\Software_Teamwork\services\qa
$env:QA_AI_GATEWAY_SMOKE = "1"
$env:AI_GATEWAY_URL = "http://localhost:8086/internal/v1/chat/completions"
$env:AI_GATEWAY_TOKEN = [Environment]::GetEnvironmentVariable('INTERNAL_SERVICE_TOKEN', 'User')
$env:AI_GATEWAY_TOKEN_HEADER = "X-Service-Token"
$env:AI_GATEWAY_PROFILE_ID = "replace-with-chat-profile-id"
$env:MODEL_ID = "replace-with-exact-profile-model"
go test ./internal/platform/modelclient -run '^TestAIGatewaySmoke$' -count=1 -v
```

Bash:

```bash
cd services/qa
export QA_AI_GATEWAY_SMOKE=1
export AI_GATEWAY_URL=http://localhost:8086/internal/v1/chat/completions
export AI_GATEWAY_TOKEN="$INTERNAL_SERVICE_TOKEN"
export AI_GATEWAY_TOKEN_HEADER=X-Service-Token
export AI_GATEWAY_PROFILE_ID=replace-with-chat-profile-id
export MODEL_ID=replace-with-exact-profile-model
go test ./internal/platform/modelclient -run '^TestAIGatewaySmoke$' -count=1 -v
```

`AI_GATEWAY_TOKEN` may be omitted when `INTERNAL_SERVICE_TOKEN` is set. Optional
`AI_GATEWAY_TIMEOUT` uses Go duration syntax and defaults to `60s`. A successful
run reports these subtests. When using the root Compose SQL seed, the initial
pair is `AI_GATEWAY_PROFILE_ID=default-chat` and
`MODEL_ID=local-placeholder-chat`; that placeholder profile still needs a
reachable compatible provider/model before the positive call can succeed.

```text
TestAIGatewaySmoke/successful_completion
TestAIGatewaySmoke/invalid_service_token
TestAIGatewaySmoke/missing_profile
```

The successful subtest logs a generated request ID. Use it to correlate QA,
AI Gateway, and controlled-provider diagnostics without logging tokens, prompts,
provider response bodies, or provider API keys.

| Symptom | Action |
| ------- | ------ |
| Smoke reports `SKIP` | Set `QA_AI_GATEWAY_SMOKE=1`; normal CI intentionally leaves it unset. |
| Token configuration is required | Set `AI_GATEWAY_TOKEN` or the fallback `INTERNAL_SERVICE_TOKEN`. |
| Profile configuration is required | Set `AI_GATEWAY_PROFILE_ID` to an enabled chat profile. |
| Successful completion returns `dependency_error` | Check AI Gateway readiness, profile credential status, provider availability, and logs for the emitted request ID. |
| Request is rejected before provider invocation | Verify token hashes, `MODEL_ID` exact-match, and that the selected profile is enabled and not deleted. |

This smoke does not start AI Gateway, create profiles, or exercise PostgreSQL QA
sessions, Gateway/Auth, Knowledge retrieval, MCP, or frontend flows.

### Optional MCP transports

`MCP_TRANSPORT` defaults to `disabled`; built-in tools still work. Runtime
configuration supports `streamable_http` MCP servers only. The stdio transport
is reserved for package-owned SDK lifecycle tests so deployed QA containers do
not try to launch local source-only helper commands.

Set `MCP_TRANSPORT=streamable_http` and provide `MCP_SERVER_URL` to merge remote
tools with the built-in registry. Optional credentials use `MCP_SERVER_TOKEN`
and `MCP_SERVER_TOKEN_HEADER`.

### MCP local integration

Use this section to verify MCP client wiring without PostgreSQL, Gateway, or a
real knowledge MCP server.

#### Automated tests (no LLM, no manual server)

From `services/qa`:

```powershell
go test ./internal/platform/mcpclient/... -v
go test ./internal/platform/toolclient/... -v
go test ./internal/platform/connectiontest/... -v
```

These cover the test-only stdio echo helper, streamable HTTP transport, tool
name prefixing, composite tool routing, and the settings connection-test path
against an in-process echo server (`internal/platform/mcpclient/testserver`).

#### Manual REPL check with the bundled echo server

`cmd/mcp-echo` is a minimal streamable HTTP MCP server with one tool: `echo`.
Use **two terminals**; the echo server blocks its terminal until you stop it.

Terminal A — start the echo server:

```powershell
cd D:\PROJECTS\Software_Teamwork\services\qa
go run ./cmd/mcp-echo
```

Expected log:

```text
MCP echo server listening on http://localhost:8099
```

Terminal B — start the agent REPL and point it at the echo server:

```powershell
cd D:\PROJECTS\Software_Teamwork\services\qa
$env:MCP_TRANSPORT = "streamable_http"
$env:MCP_SERVER_URL = "http://localhost:8099"
$env:AI_GATEWAY_URL = "http://localhost:8086/internal/v1/chat/completions"
$env:AI_GATEWAY_TOKEN = [Environment]::GetEnvironmentVariable('INTERNAL_SERVICE_TOKEN', 'User')
$env:AI_GATEWAY_TOKEN_HEADER = "X-Service-Token"
go run ./cmd/agent
```

At the `qa >>` prompt, ask which tools are available. A successful merge lists
local tools (`read_file`, `write_file`, `edit_file`) plus remote tool `echo`.

To verify `tools/call`, ask the model to invoke echo explicitly, for example:

```text
请调用 echo 工具，参数 text 设为 hello-mcp，并把原始返回结果告诉我
```

Stop the REPL with `exit` or `q`, then press `Ctrl+C` in terminal A.

#### Common issues

| Symptom | Likely cause |
| ------- | ------------ |
| `AI gateway returned HTTP 401` | `AI_GATEWAY_TOKEN` does not match AI Gateway service-token hashes. |
| `chat profile not found` | The configured QA `profileId` has not been created in AI Gateway. |
| `initialize MCP session` failed | Echo server not running, wrong URL, or agent started in the same terminal as the server. |
| Tool list has no `echo` | `MCP_TRANSPORT` or `MCP_SERVER_URL` not set in the agent terminal. |
| Agent lists `echo` but does not call it | Rephrase the request to explicitly ask for an echo tool call. |

## PostgreSQL with Docker

QA PostgreSQL runs in Docker (`postgres:16-alpine`) and schema changes are
applied with [`goose`](https://github.com/pressly/goose) `v3.27.1`, matching the
project technology baseline. Migrations are **not** mounted into
`docker-entrypoint-initdb.d`; the one-shot `migrate` service runs `goose up`
after PostgreSQL becomes healthy.

Start only the database (typical for local `go run ./cmd/server`):

```powershell
.\scripts\docker-db-up.ps1
```

```bash
./scripts/docker-db-up.sh
```

Or manually:

```powershell
docker compose -f docker-compose.db.yml up -d --build postgres
docker compose -f docker-compose.db.yml up --build migrate
```

Connection string (host port defaults to `5433`):

```text
postgres://qa_app:qa_app_dev@localhost:5433/qa_system?sslmode=disable
```

Reset the local database volume and re-apply migrations:

```powershell
docker compose -f docker-compose.db.yml down -v
.\scripts\docker-db-up.ps1
```

Apply or inspect migrations on the host with the project-pinned `goose@v3.27.1` command:

```powershell
$env:QA_DATABASE_URL = "postgres://qa_app:qa_app_dev@localhost:5433/qa_system?sslmode=disable"
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres $env:QA_DATABASE_URL up
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres $env:QA_DATABASE_URL status
```

Integration tests against the Docker database:

```powershell
$env:QA_TEST_DATABASE_URL = "postgres://qa_app:qa_app_dev@localhost:5433/qa_system?sslmode=disable"
go test ./internal/repository/... -run TestDocumentedResourceRoundTrip -count=1
```

## Run with Docker Compose

Start Auth PostgreSQL, QA PostgreSQL (+ goose migrate), Redis, Auth, QA and
Gateway. QA expects AI Gateway on the same Compose network under the
`ai-gateway` hostname; provider endpoints outside Docker should be configured
inside AI Gateway, not by pointing QA at `host.docker.internal`:

```powershell
$env:INTERNAL_SERVICE_TOKEN = [Environment]::GetEnvironmentVariable('INTERNAL_SERVICE_TOKEN', 'User')
docker compose up -d --build
```

The full stack includes `docker-compose.db.yml` via Compose `include`; QA waits
for the `migrate` job to finish before starting.

If a local Docker registry mirror cannot pull the Go/Alpine build images, use
the cached-image fallback after compiling a Linux binary on the host:

```powershell
$env:CGO_ENABLED = '0'; $env:GOOS = 'linux'; $env:GOARCH = 'amd64'
go build -o bin/qa-server ./cmd/server
Push-Location ../auth; go build -o bin/auth-server ./cmd/server; Pop-Location
Push-Location ../gateway; go build -o bin/gateway-server ./cmd/server; Pop-Location
$env:QA_DOCKERFILE = 'Dockerfile.host'
$env:AUTH_DOCKERFILE = 'Dockerfile.host'
$env:GATEWAY_DOCKERFILE = 'Dockerfile.host'
docker compose up -d --build
```

Verify public readiness:

```powershell
Invoke-RestMethod http://localhost:8080/readyz
```

Regenerate sqlc code after changing query files:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

Generated query code lives in `internal/repository/sqlc/`; SQL sources live in
`internal/repository/queries/` (sessions/messages/runs plus `settings.sql` for
runtime settings, MCP servers, and config versions). Generated types stay in the
repository adapter layer only; handlers, services, and MCP client code must not
import `internal/repository/sqlc`.

QA uses `pgx/v5` via `sqlc.yaml`; this matches the service `go.mod` baseline
while the monorepo-wide pgx version decision is tracked separately in
`docs/architecture/technology-decisions.md`.

## HTTP API

The frontend calls Gateway on port `8080`. QA's port `8084` remains reachable
for operations, but every `/internal/v1/**` request requires `X-Service-Token`;
Gateway validates Bearer sessions and injects trusted user context.

```powershell
$session = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/sessions `
  -ContentType application/json `
  -Body '{"username":"admin","password":"your-local-password"}'
$headers = @{ Authorization = "Bearer $($session.data.session.accessToken)" }
$conversation = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/qa-sessions `
  -Headers $headers -ContentType application/json `
  -Body '{"title":"联调会话"}'

Invoke-RestMethod -Method Post `
  -Uri "http://localhost:8080/api/v1/qa-sessions/$($conversation.data.id)/messages" `
  -Headers $headers -ContentType application/json `
  -Body '{"message":"请介绍可用工具"}'
```

The implemented internal resource list is maintained in
[`api/openapi.yaml`](api/openapi.yaml) and references the authoritative Gateway
contract. It includes sessions/messages, event replay, response runs, tool-call
summaries, citations, QA/LLM config versions, connection tests, retrieval tests,
and metrics.

Send `Accept: text/event-stream` to the same
`POST /api/v1/qa-sessions/{sessionId}/messages` public path to receive SSE.
Events use the documented names such as `message.created`,
`agent.iteration.started`, `tool.started`, `reasoning.step`, `answer.delta`,
`answer.completed`, and `error`; resumable events are persisted for the replay
resource. By default the AI Gateway provider call remains non-streaming, so the
completed model answer is emitted as one safe `answer.delta`; set
`AI_GATEWAY_STREAM=true` only for profiles that support streaming completions.

## Run the CLI

```bash
go run ./cmd/agent
```

The REPL remains available for Agent Loop debugging.

## Verify

```bash
go test ./...
go build ./cmd/server
go build ./cmd/agent
go build ./cmd/mcp-echo
```

For MCP-only checks without the full suite:

```powershell
go test ./internal/platform/mcpclient/... ./internal/platform/toolclient/... ./internal/platform/connectiontest/... -v
```
