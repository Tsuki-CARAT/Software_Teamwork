package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/repository/sqlc"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/service"
)

type queryExecutor interface {
	sqlc.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type userQueries interface {
	GetUserByID(ctx context.Context, id string) (sqlc.AuthUser, error)
	GetUserByUsername(ctx context.Context, username string) (sqlc.AuthUser, error)
	GetCredentialByUserID(ctx context.Context, arg sqlc.GetCredentialByUserIDParams) (sqlc.AuthCredential, error)
	GetSessionByID(ctx context.Context, id string) (sqlc.AuthSession, error)
	GetActiveSessionByTokenHash(ctx context.Context, accessTokenHash string) (sqlc.AuthSession, error)
	ListRoleCodesByUserID(ctx context.Context, userID string) ([]string, error)
	ListPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
	CreateSession(ctx context.Context, arg sqlc.CreateSessionParams) (sqlc.AuthSession, error)
	RevokeSession(ctx context.Context, arg sqlc.RevokeSessionParams) (sqlc.AuthSession, error)
}

type PostgresRepository struct {
	db      queryExecutor
	queries userQueries
}

func NewPostgresRepository(db queryExecutor) *PostgresRepository {
	return &PostgresRepository{
		db:      db,
		queries: sqlc.New(db),
	}
}

func NewPostgresRepositoryFromPool(pool *pgxpool.Pool) *PostgresRepository {
	return NewPostgresRepository(pool)
}

func newPostgresRepositoryForTest(queries userQueries) *PostgresRepository {
	return &PostgresRepository{queries: queries}
}

func (r *PostgresRepository) FindUserByID(ctx context.Context, id string) (service.UserRecord, error) {
	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		return service.UserRecord{}, classifyNoRows("find user", err)
	}
	return r.userRecord(ctx, mapUser(user))
}

func (r *PostgresRepository) FindUserByUsername(ctx context.Context, username string) (service.UserRecord, error) {
	user, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return service.UserRecord{}, classifyNoRows("find user", err)
	}
	return r.userRecord(ctx, mapUser(user))
}

func (r *PostgresRepository) FindCredentialByUserID(ctx context.Context, userID string) (service.Credential, error) {
	credential, err := r.queries.GetCredentialByUserID(ctx, sqlc.GetCredentialByUserIDParams{
		UserID:         userID,
		CredentialType: service.CredentialTypePassword,
	})
	if err != nil {
		return service.Credential{}, classifyNoRows("find credential", err)
	}
	return mapCredential(credential), nil
}

func (r *PostgresRepository) FindSessionByID(ctx context.Context, id string) (service.SessionIdentity, error) {
	session, err := r.queries.GetSessionByID(ctx, id)
	if err != nil {
		return service.SessionIdentity{}, classifyNoRows("find session", err)
	}
	return r.sessionIdentity(ctx, mapSession(session))
}

func (r *PostgresRepository) FindActiveSessionByTokenHash(ctx context.Context, tokenHash string) (service.SessionIdentity, error) {
	session, err := r.queries.GetActiveSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return service.SessionIdentity{}, classifyNoRows("find active session", err)
	}
	return r.sessionIdentity(ctx, mapSession(session))
}

func (r *PostgresRepository) CreateUserWithCredential(ctx context.Context, params service.CreateUserParams) (service.UserRecord, error) {
	if r.db == nil {
		return service.UserRecord{}, fmt.Errorf("create user with credential: repository is not configured with a database executor")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return service.UserRecord{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer rollback(ctx, tx)

	q := sqlc.New(tx)
	status := params.Status
	if status == "" {
		status = service.UserStatusActive
	}
	now := params.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          params.ID,
		Username:    params.Username,
		DisplayName: params.DisplayName,
		Email:       nullableString(params.Email),
		Phone:       nullableString(params.Phone),
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return service.UserRecord{}, service.ErrConflict
		}
		return service.UserRecord{}, fmt.Errorf("create user: %w", err)
	}

	paramsJSON := params.PasswordHashParamsJSON
	if paramsJSON == "" {
		paramsJSON = "{}"
	}
	if _, err := q.CreateCredential(ctx, sqlc.CreateCredentialParams{
		ID:                        params.PasswordCredentialID,
		UserID:                    user.ID,
		CredentialType:            service.CredentialTypePassword,
		PasswordHash:              params.PasswordHash,
		PasswordHashAlg:           params.PasswordHashAlg,
		PasswordHashParamsVersion: params.PasswordHashParamsVersion,
		Column7:                   jsonbFromString(paramsJSON),
		PasswordChangedAt:         now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}); err != nil {
		if isUniqueViolation(err) {
			return service.UserRecord{}, service.ErrConflict
		}
		return service.UserRecord{}, fmt.Errorf("create credential: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return service.UserRecord{}, fmt.Errorf("commit create user transaction: %w", err)
	}

	return service.UserRecord{User: mapUser(user), Roles: []string{}, Permissions: []string{}}, nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, params service.CreateSessionParams) (service.SessionIdentity, error) {
	issuedAt := params.IssuedAt
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	session, err := r.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:                        params.ID,
		UserID:                    params.UserID,
		AccessTokenHash:           params.AccessTokenHash,
		AccessTokenHashAlg:        params.AccessTokenHashAlg,
		AccessTokenHashKeyVersion: params.AccessTokenHashKeyVersion,
		IssuedAt:                  issuedAt,
		ExpiresAt:                 params.ExpiresAt,
		ClientIp:                  nullableString(params.ClientIP),
		UserAgent:                 nullableString(params.UserAgent),
		CreatedRequestID:          nullableString(params.RequestID),
		CreatedAt:                 issuedAt,
		UpdatedAt:                 issuedAt,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return service.SessionIdentity{}, service.ErrConflict
		}
		return service.SessionIdentity{}, fmt.Errorf("create session: %w", err)
	}
	return r.sessionIdentity(ctx, mapSession(session))
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, params service.RevokeSessionParams) (service.Session, error) {
	revokedAt := params.RevokedAt
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	session, err := r.queries.RevokeSession(ctx, sqlc.RevokeSessionParams{
		ID:               params.SessionID,
		RevokedAt:        sql.NullTime{Time: revokedAt, Valid: true},
		RevokeReason:     sql.NullString{String: params.Reason, Valid: params.Reason != ""},
		RevokedRequestID: nullableString(params.RequestID),
	})
	if err != nil {
		return service.Session{}, classifyNoRows("revoke session", err)
	}
	return mapSession(session), nil
}

func (r *PostgresRepository) userRecord(ctx context.Context, user service.User) (service.UserRecord, error) {
	roles, err := r.queries.ListRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return service.UserRecord{}, fmt.Errorf("list user roles: %w", err)
	}
	permissions, err := r.queries.ListPermissionCodesByUserID(ctx, user.ID)
	if err != nil {
		return service.UserRecord{}, fmt.Errorf("list user permissions: %w", err)
	}
	return service.UserRecord{User: user, Roles: roles, Permissions: permissions}, nil
}

func (r *PostgresRepository) sessionIdentity(ctx context.Context, session service.Session) (service.SessionIdentity, error) {
	record, err := r.FindUserByID(ctx, session.UserID)
	if err != nil {
		return service.SessionIdentity{}, err
	}
	return service.SessionIdentity{
		Session: session,
		User: service.UserSummary{
			ID:          record.ID,
			Username:    record.Username,
			Roles:       record.Roles,
			Permissions: record.Permissions,
		},
		AccessTokenHash: session.AccessTokenHash,
	}, nil
}

func mapUser(user sqlc.AuthUser) service.User {
	return service.User{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       stringPtr(user.Email),
		Phone:       stringPtr(user.Phone),
		Status:      user.Status,
		LockedUntil: timePtr(user.LockedUntil),
		LastLoginAt: timePtr(user.LastLoginAt),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		DeletedAt:   timePtr(user.DeletedAt),
	}
}

func mapCredential(credential sqlc.AuthCredential) service.Credential {
	return service.Credential{
		ID:                        credential.ID,
		UserID:                    credential.UserID,
		CredentialType:            credential.CredentialType,
		PasswordHash:              credential.PasswordHash,
		PasswordHashAlg:           credential.PasswordHashAlg,
		PasswordHashParamsVersion: credential.PasswordHashParamsVersion,
		PasswordHashParamsJSON:    normalizeJSONB(credential.PasswordHashParamsJson),
		PasswordChangedAt:         credential.PasswordChangedAt,
		PasswordExpiresAt:         timePtr(credential.PasswordExpiresAt),
		FailedAttemptCount:        credential.FailedAttemptCount,
		LastFailedAt:              timePtr(credential.LastFailedAt),
		CreatedAt:                 credential.CreatedAt,
		UpdatedAt:                 credential.UpdatedAt,
	}
}

func mapSession(session sqlc.AuthSession) service.Session {
	return service.Session{
		ID:                        session.ID,
		UserID:                    session.UserID,
		AccessTokenHash:           session.AccessTokenHash,
		AccessTokenHashAlg:        session.AccessTokenHashAlg,
		AccessTokenHashKeyVersion: session.AccessTokenHashKeyVersion,
		TokenType:                 session.TokenType,
		Status:                    session.Status,
		IssuedAt:                  session.IssuedAt,
		ExpiresAt:                 session.ExpiresAt,
		LastSeenAt:                timePtr(session.LastSeenAt),
		RevokedAt:                 timePtr(session.RevokedAt),
		RevokeReason:              stringPtr(session.RevokeReason),
		ClientIP:                  stringPtr(session.ClientIp),
		UserAgent:                 stringPtr(session.UserAgent),
		CreatedRequestID:          stringPtr(session.CreatedRequestID),
		RevokedRequestID:          stringPtr(session.RevokedRequestID),
		CreatedAt:                 session.CreatedAt,
		UpdatedAt:                 session.UpdatedAt,
	}
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func jsonbFromString(raw string) pgtype.JSONB {
	return pgtype.JSONB{Bytes: []byte(raw), Status: pgtype.Present}
}

func normalizeJSONB(value pgtype.JSONB) string {
	if value.Status != pgtype.Present || len(value.Bytes) == 0 {
		return "{}"
	}
	var decoded any
	if err := json.Unmarshal(value.Bytes, &decoded); err != nil {
		return string(value.Bytes)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return string(value.Bytes)
	}
	return string(normalized)
}

func classifyNoRows(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
