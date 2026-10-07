package db

import (
	"context"
	"database/sql"
)

type DashboardMetrics struct {
	TotalGames                   int64
	AvailableGames               int64
	TotalUsers                   int64
	ActiveLoans                  int64
	OverdueLoans                 int64
	PendingPickupsToday          int64
	ConfirmedPickupsToday        int64
	TodayPickups                 int64
	PendingBookings              int64
	ReturnsToday                 int64
	AverageBGGRating             sql.NullFloat64
	MostBorrowedGame             string
	MostBorrowedCount            int64
	GamesNeedingBGGSyncCount     int64
	GamesMissingLocationCount    int64
	GamesMissingCustomImageCount int64
}

type DashboardGame struct {
	ID   int32
	Name string
}

type DashboardData struct {
	Metrics                 DashboardMetrics
	GamesNeedingBGGSync     []DashboardGame
	GamesMissingLocation    []DashboardGame
	GamesMissingCustomImage []DashboardGame
}

func (q *Queries) GetDashboardData(ctx context.Context) (DashboardData, error) {
	var data DashboardData
	err := q.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM items),
			(SELECT COUNT(*) FROM items i
			 WHERE i.status = 'available'
			   AND NOT EXISTS (
			       SELECT 1 FROM booking_items bi
			       JOIN bookings b ON b.id = bi.booking_id
			       WHERE bi.item_id = i.id
			         AND b.status IN ('pending', 'confirmed', 'active')
			         AND b.start_date <= NOW()
			         AND b.end_date >= NOW()
			   )),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM bookings WHERE status = 'active'),
			(SELECT COUNT(*) FROM bookings WHERE status = 'active' AND end_date < CURRENT_DATE),
			(SELECT COUNT(*) FROM bookings WHERE status = 'pending' AND start_date >= CURRENT_DATE AND start_date < CURRENT_DATE + INTERVAL '1 day'),
			(SELECT COUNT(*) FROM bookings WHERE status = 'confirmed' AND start_date >= CURRENT_DATE AND start_date < CURRENT_DATE + INTERVAL '1 day'),
			(SELECT COUNT(*) FROM bookings WHERE status = 'pending'),
			(SELECT COUNT(*) FROM bookings WHERE status = 'active' AND end_date >= CURRENT_DATE AND end_date < CURRENT_DATE + INTERVAL '1 day'),
			(SELECT (AVG(bgg_rating) / 2)::float8 FROM items WHERE bgg_rating IS NOT NULL),
			COALESCE((SELECT name FROM (
				SELECT i.name, COUNT(*) AS loan_count
				FROM booking_items bi
				JOIN bookings b ON b.id = bi.booking_id
				JOIN items i ON i.id = bi.item_id
				WHERE b.status IN ('active', 'returned')
				GROUP BY i.id, i.name
				ORDER BY loan_count DESC, i.name
				LIMIT 1
			) most_borrowed), ''),
			(SELECT COUNT(*) FROM booking_items bi
			 JOIN bookings b ON b.id = bi.booking_id
			 WHERE b.status IN ('active', 'returned')
			   AND bi.item_id = (
			       SELECT bi2.item_id FROM booking_items bi2
			       JOIN bookings b2 ON b2.id = bi2.booking_id
			       JOIN items i2 ON i2.id = bi2.item_id
			       WHERE b2.status IN ('active', 'returned')
			       GROUP BY bi2.item_id, i2.name
			       ORDER BY COUNT(*) DESC, i2.name
			       LIMIT 1
			   ))
	`).Scan(
		&data.Metrics.TotalGames,
		&data.Metrics.AvailableGames,
		&data.Metrics.TotalUsers,
		&data.Metrics.ActiveLoans,
		&data.Metrics.OverdueLoans,
		&data.Metrics.PendingPickupsToday,
		&data.Metrics.ConfirmedPickupsToday,
		&data.Metrics.PendingBookings,
		&data.Metrics.ReturnsToday,
		&data.Metrics.AverageBGGRating,
		&data.Metrics.MostBorrowedGame,
		&data.Metrics.MostBorrowedCount,
	)
	if err != nil {
		return DashboardData{}, err
	}
	data.Metrics.TodayPickups = data.Metrics.PendingPickupsToday + data.Metrics.ConfirmedPickupsToday

	data.GamesNeedingBGGSync, err = q.listDashboardGames(ctx, `
		SELECT id, name, COUNT(*) OVER()
		FROM items
		WHERE bgg_rating IS NULL
		   OR NULLIF(TRIM(COALESCE(image_url, '')), '') IS NULL
		ORDER BY name
		LIMIT 8
	`, &data.Metrics.GamesNeedingBGGSyncCount)
	if err != nil {
		return DashboardData{}, err
	}
	data.GamesMissingLocation, err = q.listDashboardGames(ctx, `
		SELECT id, name, COUNT(*) OVER()
		FROM items
		WHERE NULLIF(TRIM(COALESCE(room, '')), '') IS NULL
		   OR NULLIF(TRIM(COALESCE(shelf, '')), '') IS NULL
		ORDER BY name
		LIMIT 8
	`, &data.Metrics.GamesMissingLocationCount)
	if err != nil {
		return DashboardData{}, err
	}
	data.GamesMissingCustomImage, err = q.listDashboardGames(ctx, `
		SELECT id, name, COUNT(*) OVER()
		FROM items
		WHERE custom_image_url IS NULL OR TRIM(custom_image_url) = ''
		ORDER BY name
		LIMIT 8
	`, &data.Metrics.GamesMissingCustomImageCount)
	if err != nil {
		return DashboardData{}, err
	}
	return data, nil
}

func (q *Queries) listDashboardGames(ctx context.Context, query string, count *int64) ([]DashboardGame, error) {
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	games := make([]DashboardGame, 0)
	for rows.Next() {
		var game DashboardGame
		if err := rows.Scan(&game.ID, &game.Name, count); err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, rows.Err()
}
