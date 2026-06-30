package httpapi

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

type createParserConfigRequest struct {
	Name                  string                `json:"name"`
	Backend               service.ParserBackend `json:"backend"`
	Enabled               *bool                 `json:"enabled"`
	IsDefault             *bool                 `json:"isDefault"`
	Concurrency           int                   `json:"concurrency"`
	SupportedContentTypes []string              `json:"supportedContentTypes"`
	EndpointURL           *string               `json:"endpointUrl"`
	DefaultParameters     json.RawMessage       `json:"defaultParameters"`
}
type updateParserConfigRequest struct {
	Name                  *string                `json:"name"`
	Backend               *service.ParserBackend `json:"backend"`
	Enabled               *bool                  `json:"enabled"`
	IsDefault             *bool                  `json:"isDefault"`
	Concurrency           *int                   `json:"concurrency"`
	SupportedContentTypes *[]string              `json:"supportedContentTypes"`
	EndpointURL           json.RawMessage        `json:"endpointUrl"`
	DefaultParameters     *json.RawMessage       `json:"defaultParameters"`
}
type parserConfigResponse struct {
	ID                    string                `json:"id"`
	Name                  string                `json:"name"`
	Backend               service.ParserBackend `json:"backend"`
	Enabled               bool                  `json:"enabled"`
	IsDefault             bool                  `json:"isDefault"`
	Concurrency           int                   `json:"concurrency"`
	SupportedContentTypes []string              `json:"supportedContentTypes,omitempty"`
	EndpointURL           *string               `json:"endpointUrl"`
	DefaultParameters     json.RawMessage       `json:"defaultParameters,omitempty"`
	CreatedAt             time.Time             `json:"createdAt"`
	UpdatedAt             time.Time             `json:"updatedAt"`
}

func parserConfigFromDomain(c service.ParserConfig) parserConfigResponse {
	return parserConfigResponse{c.ID, c.Name, c.Backend, c.Enabled, c.IsDefault, c.Concurrency, c.SupportedContentTypes, c.EndpointURL, c.DefaultParameters, c.CreatedAt, c.UpdatedAt}
}
func parserConfigsFromDomain(items []service.ParserConfig) []parserConfigResponse {
	out := make([]parserConfigResponse, len(items))
	for i, v := range items {
		out[i] = parserConfigFromDomain(v)
	}
	return out
}

func parseNullableString(raw json.RawMessage) (*string, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, err
	}
	return &value, true, nil
}
