package modelendpoint

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

const chatCompletionsPath = "/internal/v1/chat/completions"

// NormalizeAIGatewayChatEndpoint validates the only model egress endpoint QA
// may call directly. Provider-specific base URLs and credentials belong in AI
// Gateway profiles, not in QA runtime settings.
func NormalizeAIGatewayChatEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("must be an absolute http(s) URL")
	}
	if parsed.User != nil {
		return "", errors.New("must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("must not contain query or fragment")
	}
	if strings.TrimRight(parsed.EscapedPath(), "/") != chatCompletionsPath {
		return "", errors.New("must target AI Gateway chat completions")
	}
	if !trustedInternalHost(parsed.Hostname()) {
		return "", errors.New("host is not trusted")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func trustedInternalHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "" {
		return false
	}
	switch host {
	case "localhost", "ai-gateway":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
