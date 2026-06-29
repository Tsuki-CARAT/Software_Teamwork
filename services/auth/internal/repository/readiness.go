package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ReadinessChecker struct {
	pool *pgxpool.Pool
}

func NewReadinessChecker(pool *pgxpool.Pool) *ReadinessChecker {
	return &ReadinessChecker{pool: pool}
}

func (c *ReadinessChecker) Check(ctx context.Context) error {
	if c == nil || c.pool == nil {
		return fmt.Errorf("postgres pool is not configured")
	}
	if err := c.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}
	return nil
}
