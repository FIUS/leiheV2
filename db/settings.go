package db

import "context"

type AppSettings struct {
	RequireDeposit        bool
	ModuleBarcodesEnabled bool
}

func (q *Queries) GetAppSettings(ctx context.Context) (AppSettings, error) {
	var settings AppSettings
	err := q.db.QueryRowContext(ctx, `
		SELECT require_deposit, module_barcodes_enabled
		FROM app_settings
		WHERE id = 1
	`).Scan(&settings.RequireDeposit, &settings.ModuleBarcodesEnabled)
	return settings, err
}

func (q *Queries) UpdateAppSettings(ctx context.Context, required, barcodesEnabled bool) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO app_settings (id, require_deposit, module_barcodes_enabled)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE
		SET require_deposit = EXCLUDED.require_deposit,
		    module_barcodes_enabled = EXCLUDED.module_barcodes_enabled
	`, required, barcodesEnabled)
	return err
}
