package db

import "context"

type AppSettings struct {
	RequireDeposit        bool
	ModuleBarcodesEnabled bool
	EmailBookingReceived  bool
	EmailBookingReview    bool
	EmailBookingUpdates   bool
	IgnoredEmailAddresses string
}

func (q *Queries) GetAppSettings(ctx context.Context) (AppSettings, error) {
	var settings AppSettings
	err := q.db.QueryRowContext(ctx, `
		SELECT require_deposit, module_barcodes_enabled,
		       email_booking_received, email_booking_review,
		       email_booking_updates, ignored_email_addresses
		FROM app_settings
		WHERE id = 1
	`).Scan(
		&settings.RequireDeposit, &settings.ModuleBarcodesEnabled,
		&settings.EmailBookingReceived, &settings.EmailBookingReview,
		&settings.EmailBookingUpdates, &settings.IgnoredEmailAddresses,
	)
	return settings, err
}

func (q *Queries) UpdateAppSettings(ctx context.Context, settings AppSettings) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO app_settings (
		    id, require_deposit, module_barcodes_enabled,
		    email_booking_received, email_booking_review,
		    email_booking_updates, ignored_email_addresses
		)
		VALUES (1, $1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET require_deposit = EXCLUDED.require_deposit,
		    module_barcodes_enabled = EXCLUDED.module_barcodes_enabled,
		    email_booking_received = EXCLUDED.email_booking_received,
		    email_booking_review = EXCLUDED.email_booking_review,
		    email_booking_updates = EXCLUDED.email_booking_updates,
		    ignored_email_addresses = EXCLUDED.ignored_email_addresses
	`,
		settings.RequireDeposit, settings.ModuleBarcodesEnabled,
		settings.EmailBookingReceived, settings.EmailBookingReview,
		settings.EmailBookingUpdates, settings.IgnoredEmailAddresses,
	)
	return err
}
