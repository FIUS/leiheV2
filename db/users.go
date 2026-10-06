package db

import (
	"context"
	"database/sql"
	"strings"
)

func (q *Queries) PromoteUserToAdminByEmail(ctx context.Context, email string) error {
	result, err := q.db.ExecContext(ctx, `
		UPDATE users SET role = 'admin'
		WHERE LOWER(email) = LOWER($1)
	`, strings.TrimSpace(email))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
