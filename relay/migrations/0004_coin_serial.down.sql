DROP INDEX IF EXISTS idx_coins_serial;
ALTER TABLE relay_coins DROP COLUMN IF EXISTS serial_number;
