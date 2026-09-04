package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// PostgresRows describes the pgx result contract used by a durable store adapter.
type PostgresRows interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}
