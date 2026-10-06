package db

import "context"

type AppSettings struct {
	RequireDeposit bool
}

func (q *Queries) GetAppSettings(ctx context.Context) (AppSettings, error) {
	var settings AppSettings
	err := q.db.QueryRowContext(ctx, `
		SELECT require_deposit
		FROM app_settings
		WHERE id = 1
	`).Scan(&settings.RequireDeposit)
	return settings, err
}

func (q *Queries) UpdateRequireDeposit(ctx context.Context, required bool) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO app_settings (id, require_deposit)
		VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET require_deposit = EXCLUDED.require_deposit
	`, required)
	return err
}
