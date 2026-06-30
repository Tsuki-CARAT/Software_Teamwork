package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestWrapPostgresErrorMapsUniqueViolationToConflict(t *testing.T) {
	err := wrapPostgresError("create parser config", &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uq_parser_configs_live_name",
	})

	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
}
