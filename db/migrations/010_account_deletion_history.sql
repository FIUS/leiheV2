ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS user_deleted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE app_settings
    ADD COLUMN IF NOT EXISTS booking_history_days INT NOT NULL DEFAULT 0 CHECK (booking_history_days >= 0);

CREATE OR REPLACE FUNCTION mark_bookings_user_deleted() RETURNS trigger AS $$
BEGIN
    UPDATE bookings SET user_deleted = TRUE WHERE user_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_mark_bookings_deleted ON users;
CREATE TRIGGER users_mark_bookings_deleted
    BEFORE DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION mark_bookings_user_deleted();
