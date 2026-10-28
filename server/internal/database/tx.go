package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rift/server/internal/store"
)

// InTx runs fn inside a transaction: committed if fn returns nil, rolled back otherwise.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *store.Queries) error) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return fn(store.New(tx))
	})
}
