package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var ErrStorageShelfAlreadyExists = errors.New("storage shelf already exists")

var initialStorageShelves = []string{
	"A1L", "A1R", "A2L", "A2R", "A3L", "A3R", "A4L", "A4R",
	"B0", "B2L", "B2R", "B3L", "B3R", "B4L", "B4R",
	"C1L", "C1R", "C2L", "C2R", "C3L", "C3R", "CU",
}

func InitialStorageShelves() []string {
	return append([]string(nil), initialStorageShelves...)
}

func (q *Queries) EnsureStorageShelves(ctx context.Context) error {
	if _, err := q.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS storage_shelves (
			code VARCHAR(100) PRIMARY KEY,
			sort_order INTEGER NOT NULL
		)`); err != nil {
		return err
	}
	if _, err := q.db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_shelves_code_ci
		ON storage_shelves (LOWER(code))`); err != nil {
		return err
	}
	var hasShelves bool
	if err := q.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM storage_shelves)").Scan(&hasShelves); err != nil {
		return err
	}
	if !hasShelves {
		for index, code := range initialStorageShelves {
			if _, err := q.db.ExecContext(ctx,
				"INSERT INTO storage_shelves (code, sort_order) VALUES ($1, $2) ON CONFLICT (code) DO NOTHING",
				code, index,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *Queries) ListStorageShelves(ctx context.Context) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, "SELECT code FROM storage_shelves ORDER BY sort_order, code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shelves := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		shelves = append(shelves, code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shelves, nil
}

func (q *Queries) HasStorageShelf(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := q.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM storage_shelves WHERE LOWER(code) = LOWER($1))", code).Scan(&exists)
	return exists, err
}

func (q *Queries) AddStorageShelf(ctx context.Context, code string) error {
	return q.InTx(ctx, func(tx *Queries) error {
		var nextOrder int
		if err := tx.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), -1) + 1 FROM storage_shelves").Scan(&nextOrder); err != nil {
			return err
		}
		result, err := tx.db.ExecContext(ctx,
			"INSERT INTO storage_shelves (code, sort_order) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			code, nextOrder,
		)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return fmt.Errorf("%w: %q", ErrStorageShelfAlreadyExists, code)
		}
		return nil
	})
}

func (q *Queries) RenameStorageShelf(ctx context.Context, oldCode, newCode string) error {
	return q.InTx(ctx, func(tx *Queries) error {
		result, err := tx.db.ExecContext(ctx,
			"UPDATE storage_shelves SET code = $2 WHERE code = $1",
			oldCode, newCode,
		)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return sql.ErrNoRows
		}
		if _, err := tx.db.ExecContext(ctx, "UPDATE items SET shelf = $2 WHERE shelf = $1", oldCode, newCode); err != nil {
			return err
		}
		return nil
	})
}

func ValidateStorageShelfCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" || utf8.RuneCountInString(code) > 100 {
		return "", fmt.Errorf("shelf code must contain between 1 and 100 characters")
	}
	return code, nil
}
