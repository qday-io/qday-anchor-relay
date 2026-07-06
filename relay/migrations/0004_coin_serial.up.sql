-- Coin serial numbers：鏈上花費偵測所需（對照 examples/coin/main.go 的
-- serial number 追蹤）。成熟（ring group 湊齊）後由 scanner 產生並回填。
ALTER TABLE relay_coins ADD COLUMN serial_number TEXT;
CREATE UNIQUE INDEX idx_coins_serial ON relay_coins(serial_number) WHERE serial_number IS NOT NULL;
