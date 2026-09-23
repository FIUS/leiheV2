package db

import (
	"context"
	"database/sql"
	"fmt"
)

func (q *Queries) InTx(ctx context.Context, fn func(*Queries) error) error {
	db, ok := q.db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return fmt.Errorf("database connection does not support transactions")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
