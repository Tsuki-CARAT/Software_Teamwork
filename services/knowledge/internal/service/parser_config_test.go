package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestParserConfigAdminCRUDValidationAndSnapshot(t *testing.T) {
	repo := repository.NewMemoryRepository()
	counter := 0
	svc := service.NewWithOptions(repo, func() time.Time { return time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC) }, func(prefix string) string { counter++; return prefix + "_test_" + string(rune('0'+counter)) })
	admin := service.RequestContext{UserID: "usr_admin", Roles: []string{"admin"}, Permissions: []string{service.PermissionKnowledgeAdmin}}
	user := service.RequestContext{UserID: "usr_user", Permissions: []string{service.PermissionKnowledgeWrite}}
	if _, err := svc.CreateParserConfig(context.Background(), user, validParserInput("builtin", true)); !hasAppCode(err, service.CodeForbidden) {
		t.Fatalf("non-admin error=%v", err)
	}
	invalid := validParserInput("invalid", false)
	invalid.Concurrency = 129
	invalid.DefaultParameters = json.RawMessage(`[]`)
	if _, err := svc.CreateParserConfig(context.Background(), admin, invalid); !hasAppCode(err, service.CodeValidation) {
		t.Fatalf("invalid error=%v", err)
	}
	first, err := svc.CreateParserConfig(context.Background(), admin, validParserInput("builtin", true))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.ResolveParserConfig(context.Background(), "application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ParserConfigID != first.ID {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	secondInput := validParserInput("remote_compatible", true)
	endpoint := "https://parser.internal/v1"
	secondInput.EndpointURL = &endpoint
	second, err := svc.CreateParserConfig(context.Background(), admin, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	current, err := svc.GetParserConfig(context.Background(), admin, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.IsDefault {
		t.Fatal("old default was not cleared")
	}
	if err := svc.DeleteParserConfig(context.Background(), admin, second.ID); !hasAppCode(err, service.CodeConflict) {
		t.Fatalf("delete default error=%v", err)
	}
	if snapshot.ParserConfigID != first.ID || snapshot.Backend != service.ParserBackendBuiltin {
		t.Fatalf("historical snapshot changed: %+v", snapshot)
	}
	audits := repo.ParserAudits()
	if len(audits) != 2 {
		t.Fatalf("audits=%d", len(audits))
	}
	for _, audit := range audits {
		if string(audit.Summary) == "" || containsSensitive(string(audit.Summary)) {
			t.Fatalf("unsafe audit=%s", audit.Summary)
		}
	}
}

func TestResolveParserConfigFallsBackToBuiltinWhenEmpty(t *testing.T) {
	svc := service.New(repository.NewMemoryRepository())

	snapshot, err := svc.ResolveParserConfig(context.Background(), "application/pdf")
	if err != nil {
		t.Fatalf("ResolveParserConfig() error = %v", err)
	}
	if snapshot.ParserConfigID != "" {
		t.Fatalf("fallback parser config id = %q", snapshot.ParserConfigID)
	}
	if snapshot.Backend != service.ParserBackendBuiltin || snapshot.Concurrency != 4 {
		t.Fatalf("fallback snapshot = %+v", snapshot)
	}
	if !json.Valid(snapshot.DefaultParameters) {
		t.Fatalf("fallback default parameters = %s", snapshot.DefaultParameters)
	}
}

func TestResolveParserConfigPrefersMatchingNonDefaultOverDefaultFallback(t *testing.T) {
	repo := repository.NewMemoryRepository()
	now := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	repo.SeedParserConfig(service.ParserConfig{
		ID:                    "parser_default",
		Name:                  "Default builtin",
		Backend:               service.ParserBackendBuiltin,
		Enabled:               true,
		IsDefault:             true,
		Concurrency:           4,
		SupportedContentTypes: nil,
		DefaultParameters:     json.RawMessage(`{}`),
		CreatedAt:             now,
		UpdatedAt:             now,
	})
	repo.SeedParserConfig(service.ParserConfig{
		ID:                    "parser_images",
		Name:                  "Image OCR",
		Backend:               service.ParserBackendLocalOCR,
		Enabled:               true,
		IsDefault:             false,
		Concurrency:           2,
		SupportedContentTypes: []string{"image/*"},
		DefaultParameters:     json.RawMessage(`{"mode":"ocr"}`),
		CreatedAt:             now.Add(time.Second),
		UpdatedAt:             now.Add(time.Second),
	})
	svc := service.New(repo)

	snapshot, err := svc.ResolveParserConfig(context.Background(), "image/png")
	if err != nil {
		t.Fatalf("ResolveParserConfig() error = %v", err)
	}
	if snapshot.ParserConfigID != "parser_images" {
		t.Fatalf("parser config id = %q, want parser_images", snapshot.ParserConfigID)
	}
	if snapshot.Backend != service.ParserBackendLocalOCR || snapshot.Concurrency != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func validParserInput(backend string, isDefault bool) service.CreateParserConfigInput {
	return service.CreateParserConfigInput{Name: "Parser " + backend, Backend: service.ParserBackend(backend), Concurrency: 4, IsDefault: &isDefault, SupportedContentTypes: []string{"application/pdf"}, DefaultParameters: json.RawMessage(`{"language":"auto"}`)}
}
func hasAppCode(err error, code service.Code) bool {
	app, ok := service.Classify(err)
	return ok && app.Code == code
}
func containsSensitive(value string) bool {
	return len(value) > 0 && (json.Valid([]byte(value)) == false || contains(value, "parser.internal") || contains(value, "language"))
}
func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
