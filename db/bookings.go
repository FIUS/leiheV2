package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
)

var (
	ErrBookingNotEditable = errors.New("booking is not editable")
	ErrBookingConflict    = errors.New("booking conflict")
)

type UserBooking struct {
	ID        int32
	Token     string
	StartDate time.Time
	EndDate   time.Time
	Status    string
	CreatedAt time.Time
	ItemNames []string
}

type BookingOverview struct {
	PersonName     string
	ID             int32
	Token          string
	Email          sql.NullString
	StartDate      time.Time
	EndDate        time.Time
	Status         string
	CreatedByStaff bool
	CreatedAt      time.Time
	ItemNames      []string
}

func (q *Queries) ListBookingOverview(ctx context.Context) ([]BookingOverview, error) {
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.id, b.public_token,
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', borrower.first_name, borrower.last_name)), ''),
		                b.guest_name, b.guest_email, 'Gast'),
		       COALESCE(borrower.email, b.guest_email),
		       b.start_date, b.end_date, b.status::text,
		       b.created_by_user_id IS NOT NULL, b.created_at,
		       COALESCE(string_agg(i.name, ', ' ORDER BY i.name), '')
		FROM bookings b
		LEFT JOIN users borrower ON borrower.id = b.user_id
		LEFT JOIN booking_items bi ON bi.booking_id = b.id
		LEFT JOIN items i ON i.id = bi.item_id
		GROUP BY b.id, borrower.first_name, borrower.last_name, borrower.email
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
		if err := rows.Scan(&booking.ID, &booking.Token, &booking.PersonName,
			&booking.Email, &booking.StartDate, &booking.EndDate,
			&booking.Status, &booking.CreatedByStaff, &booking.CreatedAt, &itemNames); err != nil {
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

func (q *Queries) ReviewPendingBooking(ctx context.Context, bookingID int32, status string) error {
	if status != "confirmed" && status != "cancelled" {
		return ErrBookingNotEditable
	}
	result, err := q.db.ExecContext(ctx, `
		UPDATE bookings
		SET status = $2::booking_status
		WHERE id = $1 AND status = 'pending' AND created_by_user_id IS NULL
	`, bookingID, status)
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

func (q *Queries) CancelConfirmedBooking(ctx context.Context, bookingID int32) error {
	result, err := q.db.ExecContext(ctx, `
		UPDATE bookings
		SET status = 'cancelled'
		WHERE id = $1 AND status = 'confirmed'
	`, bookingID)
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

func (q *Queries) CancelUncollectedBookings(ctx context.Context, pickupDayStart time.Time) (int64, error) {
	result, err := q.db.ExecContext(ctx, `
		UPDATE bookings
		SET status = 'cancelled'
		WHERE status IN ('pending', 'confirmed')
		  AND start_date < $1
	`, pickupDayStart)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type StaffLoanItem struct {
	ID        int32  `json:"id"`
	Name      string `json:"name"`
	ISBN      string `json:"isbn"`
	Room      string `json:"room"`
	Shelf     string `json:"shelf"`
	Available bool   `json:"available"`
	Warning   string `json:"warning,omitempty"`
}

type StaffLoanBooking struct {
	ID         int32           `json:"id"`
	Token      string          `json:"token"`
	PersonName string          `json:"person_name"`
	Email      string          `json:"email"`
	StartDate  time.Time       `json:"start_date"`
	EndDate    time.Time       `json:"end_date"`
	Status     string          `json:"status"`
	Items      []StaffLoanItem `json:"items"`
}

func (q *Queries) SearchStaffLoanBookings(ctx context.Context, search, mode string) ([]StaffLoanBooking, error) {
	statuses := []string{"pending", "confirmed"}
	if mode == "return" {
		statuses = []string{"active"}
	}
	today := time.Now().UTC()
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	rows, err := q.db.QueryContext(ctx, `
		SELECT b.id, b.public_token,
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''),
		                b.guest_name, b.guest_email, 'Gast'),
		       COALESCE(u.email, b.guest_email, ''), b.start_date, b.end_date, b.status::text
		FROM bookings b
		LEFT JOIN users u ON u.id = b.user_id
		WHERE b.status::text = ANY($1::text[])
		  AND ($5 <> 'issue' OR (b.start_date >= $2 AND b.start_date < $3))
		  AND ($4 = '' OR b.public_token ILIKE '%' || $4 || '%'
		       OR COALESCE(u.first_name, '') ILIKE '%' || $4 || '%'
		       OR COALESCE(u.last_name, '') ILIKE '%' || $4 || '%'
		       OR CONCAT_WS(' ', u.first_name, u.last_name) ILIKE '%' || $4 || '%'
		       OR COALESCE(u.email, b.guest_email, '') ILIKE '%' || $4 || '%'
		       OR COALESCE(b.guest_name, '') ILIKE '%' || $4 || '%')
		ORDER BY b.start_date, b.created_at DESC
		LIMIT 50
	`, pq.Array(statuses), todayStart, tomorrowStart, strings.TrimSpace(search), mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bookings []StaffLoanBooking
	for rows.Next() {
		var booking StaffLoanBooking
		if err := rows.Scan(&booking.ID, &booking.Token, &booking.PersonName, &booking.Email,
			&booking.StartDate, &booking.EndDate, &booking.Status); err != nil {
			return nil, err
		}
		items, err := q.db.QueryContext(ctx, `
			SELECT i.id, i.name, COALESCE(i.isbn, ''),
			       COALESCE(i.room, ''), COALESCE(i.shelf, '')
			FROM booking_items bi JOIN items i ON i.id = bi.item_id
			WHERE bi.booking_id = $1 ORDER BY i.name
		`, booking.ID)
		if err != nil {
			return nil, err
		}
		for items.Next() {
			var item StaffLoanItem
			if err := items.Scan(&item.ID, &item.Name, &item.ISBN, &item.Room, &item.Shelf); err != nil {
				items.Close()
				return nil, err
			}
			booking.Items = append(booking.Items, item)
		}
		if err := items.Err(); err != nil {
			items.Close()
			return nil, err
		}
		if err := items.Close(); err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}
	return bookings, rows.Err()
}

func (q *Queries) FindStaffLoanItemBySystemID(ctx context.Context, id int32, endDate time.Time) (StaffLoanItem, error) {
	var item StaffLoanItem
	var statusAvailable, hasConflict bool
	err := q.db.QueryRowContext(ctx, `
		SELECT i.id, i.name, COALESCE(i.isbn, ''),
		       COALESCE(i.room, ''), COALESCE(i.shelf, ''),
		       i.status = 'available',
		       EXISTS (
		           SELECT 1
		           FROM booking_items bi
		           JOIN bookings b ON b.id = bi.booking_id
		           WHERE bi.item_id = i.id
		             AND b.status IN ('pending', 'confirmed', 'active')
		             AND b.start_date <= $2
		             AND b.end_date >= $3
		       )
		FROM items i
		WHERE i.id = $1
	`, id, endDate, time.Now()).Scan(&item.ID, &item.Name, &item.ISBN, &item.Room, &item.Shelf, &statusAvailable, &hasConflict)
	item.Available = statusAvailable && !hasConflict
	if hasConflict {
		item.Warning = "Das Spiel ist während des gewählten Zeitraums bereits reserviert oder verliehen."
	} else if !statusAvailable {
		item.Warning = "Das Spiel ist aktuell nicht als verfügbar markiert."
	}
	return item, err
}

func (q *Queries) SearchStaffLoanItems(ctx context.Context, search string, endDate time.Time) ([]StaffLoanItem, error) {
	rows, err := q.db.QueryContext(ctx, `
		SELECT i.id, i.name, COALESCE(i.isbn, ''),
		       COALESCE(i.room, ''), COALESCE(i.shelf, ''),
		       i.status = 'available',
		       EXISTS (
		           SELECT 1
		           FROM booking_items bi
		           JOIN bookings b ON b.id = bi.booking_id
		           WHERE bi.item_id = i.id
		             AND b.status IN ('pending', 'confirmed', 'active')
		             AND b.start_date <= $2
		             AND b.end_date >= $3
		       )
		FROM items i
		WHERE i.name ILIKE '%' || $1 || '%'
		ORDER BY i.name
		LIMIT 30
	`, strings.TrimSpace(search), endDate, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StaffLoanItem
	for rows.Next() {
		var item StaffLoanItem
		var statusAvailable, hasConflict bool
		if err := rows.Scan(&item.ID, &item.Name, &item.ISBN, &item.Room, &item.Shelf, &statusAvailable, &hasConflict); err != nil {
			return nil, err
		}
		item.Available = statusAvailable && !hasConflict
		if hasConflict {
			item.Warning = "Das Spiel ist während des gewählten Zeitraums bereits reserviert oder verliehen."
		} else if !statusAvailable {
			item.Warning = "Das Spiel ist aktuell nicht als verfügbar markiert."
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queries) CreateSpontaneousLoan(ctx context.Context, token string, staffID int32, guestName, guestEmail string, endDate time.Time, itemIDs []int32) (int32, error) {
	var bookingID int32
	err := q.InTx(ctx, func(tx *Queries) error {
		if len(itemIDs) == 0 {
			return ErrBookingNotEditable
		}
		startDate := time.Now()
		if err := tx.LockItemsForBooking(ctx, itemIDs); err != nil {
			return err
		}
		conflicts, err := tx.CheckItemCollisions(ctx, CheckItemCollisionsParams{
			Column1: itemIDs, StartDate: startDate, EndDate: endDate,
		})
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return ErrBookingConflict
		}
		var availableCount int
		if err := tx.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT id) FROM items
			WHERE id = ANY($1::int[]) AND status = 'available'
		`, pq.Array(itemIDs)).Scan(&availableCount); err != nil {
			return err
		}
		if availableCount != len(itemIDs) {
			return ErrBookingConflict
		}
		booking, err := tx.CreateBooking(ctx, CreateBookingParams{
			PublicToken:     token,
			CreatedByUserID: sql.NullInt32{Int32: staffID, Valid: true},
			GuestName:       sql.NullString{String: strings.TrimSpace(guestName), Valid: strings.TrimSpace(guestName) != ""},
			GuestEmail:      sql.NullString{String: strings.TrimSpace(guestEmail), Valid: strings.TrimSpace(guestEmail) != ""},
			StartDate:       startDate,
			EndDate:         endDate,
			Status:          BookingStatusActive,
		})
		if err != nil {
			return err
		}
		bookingID = booking.ID
		for _, itemID := range itemIDs {
			if err := tx.AddBookingItem(ctx, AddBookingItemParams{BookingID: booking.ID, ItemID: itemID}); err != nil {
				return err
			}
		}
		return nil
	})
	return bookingID, err
}

func (q *Queries) IssuePrebookedLoan(ctx context.Context, bookingID int32, itemIDs []int32) error {
	return q.InTx(ctx, func(tx *Queries) error {
		var status string
		if err := tx.db.QueryRowContext(ctx, `
			SELECT status::text FROM bookings WHERE id = $1 FOR UPDATE
		`, bookingID).Scan(&status); err != nil {
			return ErrBookingNotEditable
		}
		if status != "pending" && status != "confirmed" {
			return ErrBookingNotEditable
		}
		var bookedIDs []int32
		rows, err := tx.db.QueryContext(ctx, `
			SELECT item_id FROM booking_items WHERE booking_id = $1 ORDER BY item_id FOR UPDATE
		`, bookingID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int32
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			bookedIDs = append(bookedIDs, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(itemIDs) == 0 {
			return ErrBookingNotEditable
		}
		selected := make(map[int32]bool, len(itemIDs))
		for _, id := range itemIDs {
			if selected[id] {
				return ErrBookingNotEditable
			}
			selected[id] = true
		}
		for _, id := range bookedIDs {
			if selected[id] {
				delete(selected, id)
				continue
			}
			if _, err := tx.db.ExecContext(ctx, `DELETE FROM booking_items WHERE booking_id = $1 AND item_id = $2`, bookingID, id); err != nil {
				return err
			}
		}
		if len(selected) > 0 {
			return ErrBookingNotEditable
		}
		_, err = tx.db.ExecContext(ctx, `
			UPDATE bookings
			SET status = 'active',
			    start_date = CURRENT_DATE,
			    end_date = GREATEST(end_date, CURRENT_DATE)
			WHERE id = $1
		`, bookingID)
		return err
	})
}

func (q *Queries) ReturnStaffLoan(ctx context.Context, bookingID int32) error {
	result, err := q.db.ExecContext(ctx, `
		UPDATE bookings SET status = 'returned'
		WHERE id = $1 AND status = 'active'
	`, bookingID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrBookingNotEditable
	}
	return nil
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
		WHERE id = $1 AND user_id = $2 AND status IN ('pending', 'confirmed', 'active')
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
			  AND b.status IN ('pending', 'confirmed', 'active')
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
