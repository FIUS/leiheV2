package db

import (
	"context"
	"database/sql"
)

func (q *Queries) UpdateItemShelf(ctx context.Context, id int32, shelf string) error {
	result, err := q.db.ExecContext(ctx, "UPDATE items SET shelf = $2 WHERE id = $1", id, sql.NullString{String: shelf, Valid: shelf != ""})
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (q *Queries) ListStorageRooms(ctx context.Context) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, "SELECT DISTINCT TRIM(room) FROM items WHERE NULLIF(TRIM(COALESCE(room, '')), '') IS NOT NULL ORDER BY TRIM(room)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := make([]string, 0)
	for rows.Next() {
		var room string
		if err := rows.Scan(&room); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rooms, nil
}
