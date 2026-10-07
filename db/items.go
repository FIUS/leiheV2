package db

import "context"

func (q *Queries) GetItemCategories(ctx context.Context, itemID int32) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, `
		SELECT c.name
		FROM categories c
		JOIN item_categories ic ON ic.category_id = c.id
		WHERE ic.item_id = $1
		ORDER BY c.name
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (q *Queries) HasBookingToday(ctx context.Context, itemID int32) (bool, error) {
	var hasBooking bool
	err := q.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM booking_items bi
			JOIN bookings b ON b.id = bi.booking_id
			WHERE bi.item_id = $1
			  AND b.status IN ('confirmed', 'active')
			  AND b.start_date::date <= CURRENT_DATE
			  AND b.end_date::date >= CURRENT_DATE
		)
	`, itemID).Scan(&hasBooking)
	return hasBooking, err
}
