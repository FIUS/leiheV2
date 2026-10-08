package db

import (
	"context"
	"database/sql"
)

func (q *Queries) UpdateItemLocation(ctx context.Context, id int32, room, shelf string) error {
	result, err := q.db.ExecContext(ctx, "UPDATE items SET room = $2, shelf = $3 WHERE id = $1",
		id,
		sql.NullString{String: room, Valid: room != ""},
		sql.NullString{String: shelf, Valid: shelf != ""},
	)
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

func (q *Queries) UpdateItemRoomIfMissing(ctx context.Context, id int32, room string) error {
	result, err := q.db.ExecContext(ctx,
		"UPDATE items SET room = $2 WHERE id = $1 AND NULLIF(TRIM(COALESCE(room, '')), '') IS NULL AND NULLIF(TRIM(COALESCE(shelf, '')), '') IS NOT NULL",
		id,
		sql.NullString{String: room, Valid: room != ""},
	)
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
