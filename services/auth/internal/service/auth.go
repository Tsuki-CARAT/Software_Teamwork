package service

import (
	"context"
	"time"
)

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
	UserStatusLocked   = "locked"

	SessionStatusActive  = "active"
	SessionStatusExpired = "expired"
	SessionStatusRevoked = "revoked"

	CredentialTypePassword = "password"
	TokenTypeBearer        = "Bearer"
)

type User struct {
	ID          string
	Username    string
	DisplayName string
	Email       *string
	Phone       *string
	Status      string
	LockedUntil *time.Time
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type Credential struct {
	ID                        string
	UserID                    string
	CredentialType            string
	PasswordHash              string
	PasswordHashAlg           string
	PasswordHashParamsVersion string
	PasswordHashParamsJSON    string
	PasswordChangedAt         time.Time
	PasswordExpiresAt         *time.Time
	FailedAttemptCount        int32
	LastFailedAt              *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type Session struct {
	ID                        string
	UserID                    string
	AccessTokenHash           string
	AccessTokenHashAlg        string
	AccessTokenHashKeyVersion string
	TokenType                 string
	Status                    string
	IssuedAt                  time.Time
	ExpiresAt                 time.Time
	LastSeenAt                *time.Time
	RevokedAt                 *time.Time
	RevokeReason              *string
	ClientIP                  *string
	UserAgent                 *string
	CreatedRequestID          *string
	RevokedRequestID          *string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type UserSummary struct {
	ID          string
	Username    string
	Roles       []string
	Permissions []string
}

type UserRecord struct {
	User
	Roles       []string
	Permissions []string
}

type SessionIdentity struct {
	Session         Session
	User            UserSummary
	AccessTokenHash string
}

type UserPermissions struct {
	UserID      string
	Roles       []string
	Permissions []string
	UpdatedAt   time.Time
}

type CreateUserParams struct {
	ID                        string
	Username                  string
	DisplayName               string
	Email                     *string
	Phone                     *string
	Status                    string
	CreatedAt                 time.Time
	PasswordCredentialID      string
	PasswordHash              string
	PasswordHashAlg           string
	PasswordHashParamsVersion string
	PasswordHashParamsJSON    string
}

type CreateSessionParams struct {
	ID                        string
	UserID                    string
	AccessTokenHash           string
	AccessTokenHashAlg        string
	AccessTokenHashKeyVersion string
	IssuedAt                  time.Time
	ExpiresAt                 time.Time
	ClientIP                  *string
	UserAgent                 *string
	RequestID                 *string
}

type RevokeSessionParams struct {
	SessionID string
	Reason    string
	RequestID *string
	RevokedAt time.Time
}

type Repository interface {
	FindUserByID(ctx context.Context, id string) (UserRecord, error)
	FindUserByUsername(ctx context.Context, username string) (UserRecord, error)
	FindCredentialByUserID(ctx context.Context, userID string) (Credential, error)
	FindSessionByID(ctx context.Context, id string) (SessionIdentity, error)
	FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (SessionIdentity, error)
	CreateUserWithCredential(ctx context.Context, params CreateUserParams) (UserRecord, error)
	CreateSession(ctx context.Context, params CreateSessionParams) (SessionIdentity, error)
	RevokeSession(ctx context.Context, params RevokeSessionParams) (Session, error)
}
