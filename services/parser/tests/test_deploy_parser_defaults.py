from pathlib import Path

import yaml

PARSER_OPENAPI_FILES = (
    "services/parser/api/openapi.yaml",
    "docs/services/parser/api/internal.openapi.yaml",
)


def test_deploy_defaults_enable_ppstructurev3_backend():
    repo_root = Path(__file__).resolve().parents[3]
    env_example = (repo_root / "deploy/.env.example").read_text(encoding="utf-8")
    compose = (repo_root / "deploy/docker-compose.yml").read_text(encoding="utf-8")

    assert "PARSER_BACKEND=ppstructurev3" in env_example
    assert "PARSER_LOAD_BACKEND_ON_STARTUP=false" in env_example
    assert "PARSER_BACKEND: ${PARSER_BACKEND:-ppstructurev3}" in compose
    assert (
        "PARSER_LOAD_BACKEND_ON_STARTUP: ${PARSER_LOAD_BACKEND_ON_STARTUP:-false}" in compose
    )
    assert "PARSER_BACKEND=document" not in env_example
    assert "PARSER_BACKEND: ${PARSER_BACKEND:-document}" not in compose


def test_parser_openapi_matches_lightweight_parsed_document_response():
    repo_root = Path(__file__).resolve().parents[3]

    for relative_path in PARSER_OPENAPI_FILES:
        openapi = _read_repo_file(repo_root, relative_path)

        assert "required: [content, backend]" in openapi
        assert "contentLength:" not in openapi


def test_parser_openapi_documents_readiness_contract():
    repo_root = Path(__file__).resolve().parents[3]
    health_schemas = {
        relative_path: _load_health_response_schema(repo_root, relative_path)
        for relative_path in PARSER_OPENAPI_FILES
    }

    assert (
        health_schemas["services/parser/api/openapi.yaml"]
        == health_schemas["docs/services/parser/api/internal.openapi.yaml"]
    )

    health_schema = health_schemas["services/parser/api/openapi.yaml"]
    data_schema = health_schema["properties"]["data"]
    data_properties = data_schema["properties"]

    assert data_schema["required"] == ["service", "status"]
    assert data_properties["status"]["enum"] == ["ok", "ready", "not_ready"]
    assert data_properties["backend"]["nullable"] is True
    assert data_properties["reason"] == {
        "type": "string",
        "description": "Optional diagnostic for not-ready backend state.",
    }
    assert "reason" not in data_schema["required"]


def _load_health_response_schema(repo_root: Path, relative_path: str) -> dict[str, object]:
    openapi = yaml.safe_load(_read_repo_file(repo_root, relative_path))
    assert isinstance(openapi, dict)
    schema = openapi["components"]["schemas"]["HealthResponse"]
    assert isinstance(schema, dict)
    return schema


def test_parser_openapi_does_not_document_degraded_readiness_status():
    repo_root = Path(__file__).resolve().parents[3]

    for relative_path in PARSER_OPENAPI_FILES:
        openapi = _read_repo_file(repo_root, relative_path)

        assert "degraded" not in openapi


def _read_repo_file(repo_root: Path, relative_path: str) -> str:
    return (repo_root / relative_path).read_text(encoding="utf-8")
