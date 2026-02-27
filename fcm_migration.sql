ALTER TABLE users ADD COLUMN IF NOT EXISTS fcm_token TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_users_fcm_token ON users(fcm_token) WHERE fcm_token != '';
