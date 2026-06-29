package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/platform/authclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/response"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/service"
)

type AuthClient interface {
	CreateUser(ctx context.Context, requestID string, body []byte) (service.SessionResponse, error)
	CreateSession(ctx context.Context, requestID string, body []byte) (service.SessionResponse, error)
	DeleteSession(ctx context.Context, requestID string, sessionID string) error
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if s.authClient == nil {
		s.writeDependencyError(w, r, "auth client is not configured")
		return
	}
	s.handleAuthSessionResponse(w, r, s.authClient.CreateUser, http.StatusCreated)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if s.authClient == nil {
		s.writeDependencyError(w, r, "auth client is not configured")
		return
	}
	s.handleAuthSessionResponse(w, r, s.authClient.CreateSession, http.StatusOK)
}

func (s *Server) handleAuthSessionResponse(w http.ResponseWriter, r *http.Request, call func(context.Context, string, []byte) (service.SessionResponse, error), status int) {
	if s.authClient == nil || s.sessionStore == nil {
		s.writeDependencyError(w, r, "auth or session cache is not configured")
		return
	}
	body, ok := readRequestBody(w, r)
	if !ok {
		return
	}
	requestID := middleware.RequestIDFromContext(r.Context())
	result, err := call(r.Context(), requestID, body)
	if err != nil {
		s.writeAuthClientError(w, r, err)
		return
	}
	accessTokenHash, err := s.tokenHasher.Hash(result.Session.AccessToken)
	if err != nil {
		s.writeDependencyError(w, r, "auth returned an invalid session")
		return
	}
	now := time.Now().UTC()
	entry, ttl, err := service.CacheEntryFromSession(result, accessTokenHash, requestID, now)
	if err != nil {
		s.writeDependencyError(w, r, "auth returned an invalid session")
		return
	}
	if err := s.sessionStore.Put(r.Context(), entry, ttl); err != nil {
		s.writeDependencyError(w, r, "session cache is unavailable")
		return
	}
	response.WriteJSON(w, status, result, requestID)
}

func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	entry, _, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	response.WriteJSON(w, http.StatusOK, entry.UserSummary(), middleware.RequestIDFromContext(r.Context()))
}

func (s *Server) handleDeleteCurrentSession(w http.ResponseWriter, r *http.Request) {
	entry, accessTokenHash, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	if s.authClient == nil {
		s.writeDependencyError(w, r, "auth client is not configured")
		return
	}
	requestID := middleware.RequestIDFromContext(r.Context())
	if err := s.authClient.DeleteSession(r.Context(), requestID, entry.SessionID); err != nil {
		s.writeAuthClientError(w, r, err)
		return
	}
	if err := s.sessionStore.Delete(r.Context(), accessTokenHash); err != nil {
		s.writeDependencyError(w, r, "session cache is unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (service.SessionCacheEntry, string, bool) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		s.writeUnauthorized(w, r, "authentication required")
		return service.SessionCacheEntry{}, "", false
	}
	if s.sessionStore == nil {
		s.writeDependencyError(w, r, "session cache is not configured")
		return service.SessionCacheEntry{}, "", false
	}
	accessTokenHash, err := s.tokenHasher.Hash(token)
	if err != nil {
		s.writeUnauthorized(w, r, "invalid authentication")
		return service.SessionCacheEntry{}, "", false
	}
	entry, err := s.sessionStore.Get(r.Context(), accessTokenHash)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) || errors.Is(err, service.ErrSessionInvalid) {
			s.writeUnauthorized(w, r, "invalid authentication")
			return service.SessionCacheEntry{}, "", false
		}
		s.writeDependencyError(w, r, "session cache is unavailable")
		return service.SessionCacheEntry{}, "", false
	}
	if err := entry.Validate(accessTokenHash, time.Now().UTC()); err != nil {
		s.writeUnauthorized(w, r, "invalid authentication")
		return service.SessionCacheEntry{}, "", false
	}
	return entry, accessTokenHash, true
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

func readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.ErrorDetail{
			Code:      response.CodeValidation,
			Message:   "request body is invalid",
			RequestID: middleware.RequestIDFromContext(r.Context()),
		})
		return nil, false
	}
	return body, true
}

func (s *Server) writeAuthClientError(w http.ResponseWriter, r *http.Request, err error) {
	var remote *authclient.RemoteError
	if errors.As(err, &remote) {
		if remote.Status < http.StatusBadRequest || remote.Status >= http.StatusInternalServerError {
			s.writeDependencyError(w, r, "auth service is unavailable")
			return
		}
		response.WriteError(w, remote.Status, response.ErrorDetail{
			Code:      downstreamErrorCode(remote.Status),
			Message:   authClientErrorMessage(remote.Status),
			RequestID: middleware.RequestIDFromContext(r.Context()),
		})
		return
	}
	s.writeDependencyError(w, r, "auth service is unavailable")
}

func authClientErrorMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "request validation failed"
	case http.StatusUnauthorized:
		return "invalid authentication"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate limited"
	default:
		if text := http.StatusText(status); text != "" {
			return text
		}
		return "request failed"
	}
}

func (s *Server) writeUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	response.WriteError(w, http.StatusUnauthorized, response.ErrorDetail{
		Code:      response.CodeUnauthorized,
		Message:   message,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}

func (s *Server) writeDependencyError(w http.ResponseWriter, r *http.Request, message string) {
	response.WriteError(w, http.StatusBadGateway, response.ErrorDetail{
		Code:      response.CodeDependency,
		Message:   message,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}
