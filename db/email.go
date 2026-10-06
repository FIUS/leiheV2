package db

import (
	"context"
	"database/sql"
	"time"
)

type BookingEmailDetails struct {
	Token      string
	StartDate  time.Time
	EndDate    time.Time
	Status     string
	GuestName  sql.NullString
	GuestEmail sql.NullString
	FirstName  string
	Email      sql.NullString
	ItemNames  []string
}

func (q *Queries) CreateEmailVerificationToken(ctx context.Context, tokenHash string, userID int32, expiresAt time.Time) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, userID, expiresAt)
	return err
}

func (q *Queries) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string) (int32, error) {
	var userID int32
	err := q.db.QueryRowContext(ctx, `
		DELETE FROM email_verification_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	return userID, err
}

func (q *Queries) MarkUserEmailVerified(ctx context.Context, userID int32) error {
	_, err := q.db.ExecContext(ctx, `UPDATE users SET email_verified = TRUE WHERE id = $1`, userID)
	return err
}

func (q *Queries) GetBookingEmailDetails(ctx context.Context, publicToken string) (BookingEmailDetails, error) {
	var details BookingEmailDetails
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.public_token, b.start_date, b.end_date, b.status::text,
		       b.guest_name, b.guest_email, COALESCE(u.first_name, b.guest_name, ''),
		       COALESCE(u.email, b.guest_email), i.name
		FROM bookings b
		JOIN booking_items bi ON bi.booking_id = b.id
		JOIN items i ON i.id = bi.item_id
		LEFT JOIN users u ON u.id = b.user_id
		WHERE b.public_token = $1
		ORDER BY i.name
	`, publicToken)
	if err != nil {
		return details, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemName string
		if err := rows.Scan(&details.Token, &details.StartDate, &details.EndDate,
			&details.Status, &details.GuestName, &details.GuestEmail, &details.FirstName,
			&details.Email, &itemName); err != nil {
			return details, err
		}
		details.ItemNames = append(details.ItemNames, itemName)
	}
	return details, rows.Err()
}

func (q *Queries) GetBookingEmailDetailsByID(ctx context.Context, bookingID int32) (BookingEmailDetails, error) {
	var details BookingEmailDetails
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.public_token, b.start_date, b.end_date, b.status::text,
		       b.guest_name, b.guest_email, COALESCE(u.first_name, b.guest_name, ''),
		       COALESCE(u.email, b.guest_email), i.name
		FROM bookings b
		JOIN booking_items bi ON bi.booking_id = b.id
		JOIN items i ON i.id = bi.item_id
		LEFT JOIN users u ON u.id = b.user_id
		WHERE b.id = $1
		ORDER BY i.name
	`, bookingID)
	if err != nil {
		return details, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemName string
		if err := rows.Scan(&details.Token, &details.StartDate, &details.EndDate,
			&details.Status, &details.GuestName, &details.GuestEmail, &details.FirstName,
			&details.Email, &itemName); err != nil {
			return details, err
		}
		details.ItemNames = append(details.ItemNames, itemName)
	}
	return details, rows.Err()
}
