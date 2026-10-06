ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'confirmed';

CREATE INDEX IF NOT EXISTS idx_bookings_pending_start_date
    ON bookings(start_date)
    WHERE status = 'pending';
