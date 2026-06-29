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

Temporary direct DeepSeek variables:

| Variable            | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `DEEPSEEK_API_KEY`  | Required API key; read from the environment only.          |
| `DEEPSEEK_BASE_URL` | Optional base URL; defaults to `https://api.deepseek.com`. |
| `MODEL_ID`          | Model name; defaults to `deepseek-v4-pro`.                 |

The client appends `/chat/completions` unless the configured URL already ends
with that path. Production can override the direct provider configuration with
`AI_GATEWAY_URL`, `AI_GATEWAY_TOKEN`, and `AI_GATEWAY_TOKEN_HEADER` so QA calls
the project-owned AI Gateway instead.

On Windows, user-level environment variables set after PowerShell started are
not automatically copied into that existing process. Import them without
printing their values before running the agent:

```powershell
$env:DEEPSEEK_API_KEY = [Environment]::GetEnvironmentVariable('DEEPSEEK_API_KEY', 'User')
$env:DEEPSEEK_BASE_URL = [Environment]::GetEnvironmentVariable('DEEPSEEK_BASE_URL', 'User')
```

### Optional MCP transports

`MCP_TRANSPORT` defaults to `disabled`; built-in tools still work. Set it to
`stdio` or `streamable_http` to merge remote tools with the built-in registry.

#### Stdio

| Variable               | Description                                               |
| ---------------------- | --------------------------------------------------------- |
| `MCP_TRANSPORT`        | `stdio`.                                                  |
| `MCP_SERVER_COMMAND`   | Executable used to start the MCP server.                  |
| `MCP_SERVER_ARGS_JSON` | JSON string array of arguments; no shell parsing is used. |

Example:

```powershell
$env:MCP_SERVER_COMMAND = "python"
$env:MCP_SERVER_ARGS_JSON = '["D:/path/to/server.py"]'
```

The child server's stdout is reserved for newline-delimited MCP JSON-RPC.
Diagnostics must be written to stderr.

#### Streamable HTTP

Set `MCP_TRANSPORT=streamable_http` and provide `MCP_SERVER_URL`. Optional
credentials use `MCP_SERVER_TOKEN` and `MCP_SERVER_TOKEN_HEADER`.

## Run with Docker Compose

Import the user-level DeepSeek variables into the current PowerShell process
without printing them, then start Auth PostgreSQL, QA PostgreSQL, Redis, Auth,
QA and Gateway:

```powershell
$env:DEEPSEEK_API_KEY = [Environment]::GetEnvironmentVariable('DEEPSEEK_API_KEY', 'User')
$env:DEEPSEEK_BASE_URL = [Environment]::GetEnvironmentVariable('DEEPSEEK_BASE_URL', 'User')
docker compose up -d --build
```

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

The migration under `migrations/` is applied by PostgreSQL only when the named
volume is created for the first time. For a disposable local rebuild:

```powershell
docker compose down -v
docker compose up -d --build
```

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
resource. The provider call itself remains non-streaming, so the completed model
answer is currently emitted as one safe `answer.delta`.

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
```
