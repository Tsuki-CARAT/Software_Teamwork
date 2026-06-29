package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/repository/sqlc"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/service"
)

func TestFindUserByIDIncludesRolesAndPermissions(t *testing.T) {
	queries := newFakeQueries()
	repo := newPostgresRepositoryForTest(queries)

	record, err := repo.FindUserByID(context.Background(), "usr_123")
	if err != nil {
		t.Fatalf("FindUserByID() error = %v", err)
	}
	if record.ID != "usr_123" || record.Username != "alice" {
		t.Fatalf("record = %+v", record)
	}
	if got, want := strings.Join(record.Roles, ","), "admin,member"; got != want {
		t.Fatalf("roles = %q", got)
	}
	if got, want := strings.Join(record.Permissions, ","), "document:upload,knowledge:read"; got != want {
		t.Fatalf("permissions = %q", got)
	}
}

func TestFindCredentialByUserIDMapsJSON(t *testing.T) {
	queries := newFakeQueries()
	repo := newPostgresRepositoryForTest(queries)

	credential, err := repo.FindCredentialByUserID(context.Background(), "usr_123")
	if err != nil {
		t.Fatalf("FindCredentialByUserID() error = %v", err)
	}
	if credential.PasswordHashParamsJSON != `{"memory":65536}` {
		t.Fatalf("PasswordHashParamsJSON = %q", credential.PasswordHashParamsJSON)
	}
}

func TestFindSessionByIDIncludesUserSummary(t *testing.T) {
	queries := newFakeQueries()
	repo := newPostgresRepositoryForTest(queries)

	identity, err := repo.FindSessionByID(context.Background(), "sess_123")
	if err != nil {
		t.Fatalf("FindSessionByID() error = %v", err)
	}
	if identity.Session.ID != "sess_123" || identity.Session.AccessTokenHash != "hmac-sha256:v1:abc" {
		t.Fatalf("session = %+v", identity.Session)
	}
	if identity.User.ID != "usr_123" || identity.User.Username != "alice" {
		t.Fatalf("user = %+v", identity.User)
	}
	if got, want := strings.Join(identity.User.Permissions, ","), "document:upload,knowledge:read"; got != want {
		t.Fatalf("permissions = %q", got)
	}
}

func TestFindUserNotFound(t *testing.T) {
	queries := newFakeQueries()
	repo := newPostgresRepositoryForTest(queries)

	_, err := repo.FindUserByID(context.Background(), "usr_missing")
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestQueriesDoNotUseSelectStar(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("queries", "auth.sql"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "select *") {
		t.Fatalf("query file uses SELECT *")
	}
}

type fakeQueries struct {
	now time.Time
}

func newFakeQueries() *fakeQueries {
	return &fakeQueries{now: time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)}
}

func (q *fakeQueries) GetUserByID(_ context.Context, id string) (sqlc.AuthUser, error) {
	if id != "usr_123" {
		return sqlc.AuthUser{}, pgx.ErrNoRows
	}
	return q.user(), nil
}

func (q *fakeQueries) GetUserByUsername(_ context.Context, username string) (sqlc.AuthUser, error) {
	if username != "alice" {
		return sqlc.AuthUser{}, pgx.ErrNoRows
	}
	return q.user(), nil
}

func (q *fakeQueries) GetCredentialByUserID(_ context.Context, arg sqlc.GetCredentialByUserIDParams) (sqlc.AuthCredential, error) {
	if arg.UserID != "usr_123" || arg.CredentialType != service.CredentialTypePassword {
		return sqlc.AuthCredential{}, pgx.ErrNoRows
	}
	return sqlc.AuthCredential{
		ID:                        "cred_123",
		UserID:                    "usr_123",
		CredentialType:            service.CredentialTypePassword,
		PasswordHash:              "$argon2id$...",
		PasswordHashAlg:           "argon2id",
		PasswordHashParamsVersion: "argon2id-v1",
		PasswordHashParamsJson:    pgtype.JSONB{Bytes: []byte(`{"memory":65536}`), Status: pgtype.Present},
		PasswordChangedAt:         q.now,
		FailedAttemptCount:        0,
		CreatedAt:                 q.now,
		UpdatedAt:                 q.now,
	}, nil
}

func (q *fakeQueries) GetSessionByID(_ context.Context, id string) (sqlc.AuthSession, error) {
	if id != "sess_123" {
		return sqlc.AuthSession{}, pgx.ErrNoRows
	}
	return q.session(), nil
}

func (q *fakeQueries) GetActiveSessionByTokenHash(_ context.Context, accessTokenHash string) (sqlc.AuthSession, error) {
	if accessTokenHash != "hmac-sha256:v1:abc" {
		return sqlc.AuthSession{}, pgx.ErrNoRows
	}
	return q.session(), nil
}

func (q *fakeQueries) ListRoleCodesByUserID(_ context.Context, userID string) ([]string, error) {
	if userID != "usr_123" {
		return nil, nil
	}
	return []string{"admin", "member"}, nil
}

func (q *fakeQueries) ListPermissionCodesByUserID(_ context.Context, userID string) ([]string, error) {
	if userID != "usr_123" {
		return nil, nil
	}
	return []string{"document:upload", "knowledge:read"}, nil
}

func (q *fakeQueries) CreateSession(context.Context, sqlc.CreateSessionParams) (sqlc.AuthSession, error) {
	return q.session(), nil
}

func (q *fakeQueries) RevokeSession(_ context.Context, arg sqlc.RevokeSessionParams) (sqlc.AuthSession, error) {
	if arg.ID != "sess_123" {
		return sqlc.AuthSession{}, pgx.ErrNoRows
	}
	session := q.session()
	session.Status = service.SessionStatusRevoked
	session.RevokedAt = arg.RevokedAt
	session.RevokeReason = arg.RevokeReason
	session.RevokedRequestID = arg.RevokedRequestID
	return session, nil
}

func (q *fakeQueries) user() sqlc.AuthUser {
	return sqlc.AuthUser{
		ID:          "usr_123",
		Username:    "alice",
		DisplayName: "Alice",
		Status:      service.UserStatusActive,
		CreatedAt:   q.now,
		UpdatedAt:   q.now,
	}
}

func (q *fakeQueries) session() sqlc.AuthSession {
	return sqlc.AuthSession{
		ID:                        "sess_123",
		UserID:                    "usr_123",
		AccessTokenHash:           "hmac-sha256:v1:abc",
		AccessTokenHashAlg:        "hmac-sha256",
		AccessTokenHashKeyVersion: "v1",
		TokenType:                 service.TokenTypeBearer,
		Status:                    service.SessionStatusActive,
		IssuedAt:                  q.now,
		ExpiresAt:                 q.now.Add(time.Hour),
		CreatedRequestID:          sql.NullString{String: "req_123", Valid: true},
		CreatedAt:                 q.now,
		UpdatedAt:                 q.now,
	}
}
