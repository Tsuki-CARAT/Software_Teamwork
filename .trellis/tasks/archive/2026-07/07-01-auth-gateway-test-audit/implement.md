# Implementation Plan

## Checklist

- [x] Create `docs/tests/0701/auth-gateway-test-report.md` with source mapping,
      taxonomy, planned cases, and result sections.
- [x] Run service-local Auth checks:
      `cd services/auth && go test ./...`
      `cd services/auth && go build ./cmd/server`
- [x] Run service-local Gateway checks:
      `cd services/gateway && go test ./...`
      `cd services/gateway && go build ./cmd/server`
- [x] Run Gateway contract checks:
      `python3 -m unittest scripts.tests.test_verify_gateway_active_api`
      `python3 scripts/verify_gateway_active_api.py`
- [x] Run deploy/Compose config validation:
      `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet`
      and optional AI profile config if local Docker supports it.
- [x] Attempt Auth/Gateway/Redis smoke through `deploy/docker-compose.yml`.
- [x] Record results and any discovered test ideas in the report.
- [x] Run `git diff --check`.

## Validation Notes

- If Docker is unavailable or Compose startup fails for environmental reasons,
  record the exact failure and keep the smoke as an explicit residual risk.
- If a command downloads Go modules, allow normal Go module behavior unless the
  environment blocks network access; record such failures separately from code
  failures.
- Do not add broad new infrastructure unless the current commands show a clear
  gap that cannot be reported honestly.
