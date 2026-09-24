package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrBookingNotEditable = errors.New("booking is not editable")
var ErrBookingConflict = errors.New("booking conflict")

type UserBooking struct {
	ID        int32
	Token     string
	StartDate time.Time
	EndDate   time.Time
	Status    string
	CreatedAt time.Time
	ItemNames []string
}

type BookingEmailDetails struct {
	Token      string
	StartDate  time.Time
	EndDate    time.Time
	Status     string
	GuestName  sql.NullString
	GuestEmail sql.NullString
	ItemNames  []string
}

type BookingOverview struct {
	ID         int32
	Token      string
	GuestName  sql.NullString
	GuestEmail sql.NullString
	StartDate  time.Time
	EndDate    time.Time
	Status     string
	CreatedAt  time.Time
	ItemNames  []string
}

func (q *Queries) ListBookingOverview(ctx context.Context) ([]BookingOverview, error) {
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.id, b.public_token, b.guest_name, b.guest_email,
		       b.start_date, b.end_date, b.status::text, b.created_at,
		       COALESCE(string_agg(i.name, ', ' ORDER BY i.name), '')
		FROM bookings b
		LEFT JOIN booking_items bi ON bi.booking_id = b.id
		LEFT JOIN items i ON i.id = bi.item_id
		GROUP BY b.id
		ORDER BY b.start_date DESC, b.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []BookingOverview
	for rows.Next() {
		var itemNames string
		var booking BookingOverview
		if err := rows.Scan(&booking.ID, &booking.Token, &booking.GuestName,
			&booking.GuestEmail, &booking.StartDate, &booking.EndDate,
			&booking.Status, &booking.CreatedAt, &itemNames); err != nil {
			return nil, err
		}
		if itemNames != "" {
			booking.ItemNames = strings.Split(itemNames, ", ")
		}
		result = append(result, booking)
	}
	return result, rows.Err()
}

func (q *Queries) UpdateBookingStatusByID(ctx context.Context, id int32, status string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE bookings SET status = $2 WHERE id = $1`, id, status)
	return err
}

func (q *Queries) ListBookingsForUser(ctx context.Context, userID int32) ([]UserBooking, error) {
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.id, b.public_token, b.start_date, b.end_date,
		       b.status::text, b.created_at,
		       COALESCE(string_agg(i.name, ', ' ORDER BY i.name), '')
		FROM bookings b
		LEFT JOIN booking_items bi ON bi.booking_id = b.id
		LEFT JOIN items i ON i.id = bi.item_id
		WHERE b.user_id = $1
		GROUP BY b.id
		ORDER BY b.start_date DESC, b.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []UserBooking
	for rows.Next() {
		var itemNames string
		var booking UserBooking
		if err := rows.Scan(&booking.ID, &booking.Token, &booking.StartDate,
			&booking.EndDate, &booking.Status, &booking.CreatedAt, &itemNames); err != nil {
			return nil, err
		}
		if itemNames != "" {
			booking.ItemNames = strings.Split(itemNames, ", ")
		}
		result = append(result, booking)
	}
	return result, rows.Err()
}

func (q *Queries) CancelUserBooking(ctx context.Context, userID, bookingID int32) error {
	result, err := q.db.ExecContext(ctx, `
		UPDATE bookings
		SET status = 'cancelled'
		WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'active')
	`, bookingID, userID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrBookingNotEditable
	}
	return nil
}

func (q *Queries) UpdateUserBookingDates(ctx context.Context, userID, bookingID int32, startDate, endDate time.Time) error {
	return q.InTx(ctx, func(tx *Queries) error {
		var itemIDs []int32
		rows, err := tx.db.QueryContext(ctx, `
			SELECT item_id
			FROM booking_items
			WHERE booking_id = $1
			FOR UPDATE
		`, bookingID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var itemID int32
			if err := rows.Scan(&itemID); err != nil {
				rows.Close()
				return err
			}
			itemIDs = append(itemIDs, itemID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(itemIDs) == 0 {
			return ErrBookingNotEditable
		}

		for _, itemID := range itemIDs {
			if _, err := tx.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, itemID); err != nil {
				return err
			}
		}

		var status string
		if err := tx.db.QueryRowContext(ctx, `
			SELECT status::text
			FROM bookings
			WHERE id = $1 AND user_id = $2
			FOR UPDATE
		`, bookingID, userID).Scan(&status); err != nil {
			return ErrBookingNotEditable
		}
		if status != "pending" {
			return ErrBookingNotEditable
		}

		var conflicts int
		if err := tx.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM booking_items bi
			JOIN bookings b ON b.id = bi.booking_id
			WHERE bi.item_id = ANY($1::int[])
			  AND b.id <> $2
			  AND b.status IN ('pending', 'active')
			  AND b.start_date <= $4
			  AND b.end_date >= $3
		`, itemIDs, bookingID, startDate, endDate).Scan(&conflicts); err != nil {
			return err
		}
		if conflicts > 0 {
			return ErrBookingConflict
		}
		_, err = tx.db.ExecContext(ctx, `
			UPDATE bookings
			SET start_date = $3, end_date = $4
			WHERE id = $1 AND user_id = $2 AND status = 'pending'
		`, bookingID, userID, startDate, endDate)
		return err
	})
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

func (q *Queries) PromoteUserToAdminByEmail(ctx context.Context, email string) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE users SET role = 'admin'
		WHERE LOWER(email) = LOWER($1)
	`, strings.TrimSpace(email))
	return err
}

func (q *Queries) GetBookingEmailDetails(ctx context.Context, publicToken string) (BookingEmailDetails, error) {
	var details BookingEmailDetails
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.public_token, b.start_date, b.end_date, b.status::text,
		       b.guest_name, b.guest_email, i.name
		FROM bookings b
		JOIN booking_items bi ON bi.booking_id = b.id
		JOIN items i ON i.id = bi.item_id
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
			&details.Status, &details.GuestName, &details.GuestEmail, &itemName); err != nil {
			return details, err
		}
		details.ItemNames = append(details.ItemNames, itemName)
	}
	return details, rows.Err()
}
