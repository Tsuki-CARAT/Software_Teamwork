package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
)

func (s *Service) ListParserConfigs(ctx context.Context, reqCtx RequestContext, enabled *bool) (ParserConfigList, error) {
	if err := requireParserAdmin(reqCtx); err != nil {
		return ParserConfigList{}, err
	}
	items, err := s.repo.ListParserConfigs(ctx, enabled)
	if err != nil {
		return ParserConfigList{}, repositoryError(err)
	}
	return ParserConfigList{Items: items}, nil
}

func (s *Service) GetParserConfig(ctx context.Context, reqCtx RequestContext, id string) (ParserConfig, error) {
	if err := requireParserAdmin(reqCtx); err != nil {
		return ParserConfig{}, err
	}
	if strings.TrimSpace(id) == "" {
		return ParserConfig{}, ValidationError("request validation failed", map[string]string{"parserConfigId": "is required"})
	}
	config, err := s.repo.GetParserConfig(ctx, id)
	if err != nil {
		return ParserConfig{}, repositoryError(err)
	}
	return config, nil
}

func (s *Service) CreateParserConfig(ctx context.Context, reqCtx RequestContext, input CreateParserConfigInput) (ParserConfig, error) {
	if err := requireParserAdmin(reqCtx); err != nil {
		return ParserConfig{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	isDefault := false
	if input.IsDefault != nil {
		isDefault = *input.IsDefault
	}
	config := ParserConfig{
		ID: s.newID("parser_config"), Name: strings.TrimSpace(input.Name), Backend: input.Backend,
		Enabled: enabled, IsDefault: isDefault, Concurrency: input.Concurrency,
		SupportedContentTypes: normalizeContentTypes(input.SupportedContentTypes),
		EndpointURL:           normalizeEndpoint(input.EndpointURL), DefaultParameters: normalizeParameters(input.DefaultParameters),
		CreatedAt: s.now(), UpdatedAt: s.now(),
	}
	if fields := validateParserConfig(config); len(fields) > 0 {
		return ParserConfig{}, ValidationError("request validation failed", fields)
	}
	audit := s.parserAudit(reqCtx, config.ID, "created", []string{"configuration"})
	created, err := s.repo.CreateParserConfig(ctx, config, audit)
	if err != nil {
		return ParserConfig{}, repositoryError(err)
	}
	return created, nil
}

func (s *Service) UpdateParserConfig(ctx context.Context, reqCtx RequestContext, input UpdateParserConfigInput) (ParserConfig, error) {
	if err := requireParserAdmin(reqCtx); err != nil {
		return ParserConfig{}, err
	}
	current, err := s.repo.GetParserConfig(ctx, input.ID)
	if err != nil {
		return ParserConfig{}, repositoryError(err)
	}
	changed := make([]string, 0, 8)
	if input.Name != nil {
		current.Name = strings.TrimSpace(*input.Name)
		changed = append(changed, "name")
	}
	if input.Backend != nil {
		current.Backend = *input.Backend
		changed = append(changed, "backend")
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
		changed = append(changed, "enabled")
	}
	if input.IsDefault != nil {
		if current.IsDefault && !*input.IsDefault {
			return ParserConfig{}, ConflictError("replace the default parser config before clearing it", nil)
		}
		current.IsDefault = *input.IsDefault
		changed = append(changed, "default")
	}
	if input.Concurrency != nil {
		current.Concurrency = *input.Concurrency
		changed = append(changed, "concurrency")
	}
	if input.SupportedContentTypes != nil {
		current.SupportedContentTypes = normalizeContentTypes(*input.SupportedContentTypes)
		changed = append(changed, "content_types")
	}
	if input.EndpointURL != nil {
		current.EndpointURL = normalizeEndpoint(*input.EndpointURL)
		changed = append(changed, "endpoint")
	}
	if input.DefaultParameters != nil {
		current.DefaultParameters = normalizeParameters(*input.DefaultParameters)
		changed = append(changed, "parameters")
	}
	if len(changed) == 0 {
		return ParserConfig{}, ValidationError("request validation failed", map[string]string{"body": "must contain at least one field"})
	}
	current.UpdatedAt = s.now()
	if fields := validateParserConfig(current); len(fields) > 0 {
		return ParserConfig{}, ValidationError("request validation failed", fields)
	}
	sort.Strings(changed)
	updated, err := s.repo.UpdateParserConfig(ctx, current, s.parserAudit(reqCtx, current.ID, "updated", changed))
	if err != nil {
		return ParserConfig{}, repositoryError(err)
	}
	return updated, nil
}

func (s *Service) DeleteParserConfig(ctx context.Context, reqCtx RequestContext, id string) error {
	if err := requireParserAdmin(reqCtx); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ValidationError("request validation failed", map[string]string{"parserConfigId": "is required"})
	}
	now := s.now()
	if err := s.repo.SoftDeleteParserConfig(ctx, id, now, s.parserAudit(reqCtx, id, "disabled", []string{"enabled"})); err != nil {
		return repositoryError(err)
	}
	return nil
}

func (s *Service) ResolveParserConfig(ctx context.Context, contentType string) (ParserConfigSnapshot, error) {
	config, err := s.repo.GetEffectiveParserConfig(ctx, strings.TrimSpace(contentType))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return defaultBuiltinParserSnapshot(), nil
		}
		return ParserConfigSnapshot{}, repositoryError(err)
	}
	if fields := validateParserConfig(config); len(fields) > 0 {
		return ParserConfigSnapshot{}, ConflictError("effective parser config is invalid", nil)
	}
	return ParserConfigSnapshot{ParserConfigID: config.ID, Backend: config.Backend, Concurrency: config.Concurrency,
		SupportedContentTypes: append([]string(nil), config.SupportedContentTypes...), EndpointURL: cloneString(config.EndpointURL),
		DefaultParameters: cloneRaw(config.DefaultParameters)}, nil
}

func defaultBuiltinParserSnapshot() ParserConfigSnapshot {
	return ParserConfigSnapshot{
		Backend:               ParserBackendBuiltin,
		Concurrency:           4,
		SupportedContentTypes: []string{"application/pdf", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/markdown", "text/plain"},
		DefaultParameters:     json.RawMessage(`{}`),
	}
}

func marshalParserConfigSnapshot(snapshot ParserConfigSnapshot) (json.RawMessage, error) {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

func requireParserAdmin(reqCtx RequestContext) error {
	if strings.TrimSpace(reqCtx.UserID) == "" {
		return UnauthorizedError()
	}
	if hasAdminRole(reqCtx.Roles) ||
		hasPermission(reqCtx.Permissions, PermissionSystemAdmin) ||
		hasPermission(reqCtx.Permissions, PermissionKnowledgeAdmin) ||
		hasPermission(reqCtx.Permissions, PermissionAdminParserConfig) {
		return nil
	}
	return ForbiddenError("knowledge administration permission is required")
}

func validateParserConfig(config ParserConfig) map[string]string {
	fields := map[string]string{}
	if config.Name == "" {
		fields["name"] = "is required"
	} else if len(config.Name) > 120 {
		fields["name"] = "must be at most 120 characters"
	}
	switch config.Backend {
	case ParserBackendBuiltin, ParserBackendTika, ParserBackendUnstructured, ParserBackendLocalOCR, ParserBackendRemoteCompatible:
	default:
		fields["backend"] = "is not supported"
	}
	if config.Concurrency < 1 || config.Concurrency > 128 {
		fields["concurrency"] = "must be between 1 and 128"
	}
	if config.IsDefault && !config.Enabled {
		fields["isDefault"] = "default config must be enabled"
	}
	if !validParameterObject(config.DefaultParameters) {
		fields["defaultParameters"] = "must be a valid JSON object"
	}
	if config.Backend == ParserBackendRemoteCompatible {
		if config.EndpointURL == nil {
			fields["endpointUrl"] = "is required for remote_compatible backend"
		} else if !validEndpoint(*config.EndpointURL) {
			fields["endpointUrl"] = "must be an absolute http or https URI without credentials"
		}
	} else if config.EndpointURL != nil && !validEndpoint(*config.EndpointURL) {
		fields["endpointUrl"] = "must be an absolute http or https URI without credentials"
	}
	for _, value := range config.SupportedContentTypes {
		if !strings.Contains(value, "/") {
			fields["supportedContentTypes"] = "must contain valid media types"
			break
		}
	}
	return fields
}

func validParameterObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return value != nil
}

func validEndpoint(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func normalizeEndpoint(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
func normalizeParameters(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return cloneRaw(value)
}
func normalizeContentTypes(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func (s *Service) parserAudit(reqCtx RequestContext, id, action string, changed []string) ParserConfigAudit {
	summary, _ := json.Marshal(map[string]any{"action": action, "changedFields": changed})
	return ParserConfigAudit{ID: s.newID("audit"), ParserConfigID: id, ActorUserID: reqCtx.UserID, Action: action, Summary: summary, CreatedAt: s.now()}
}
