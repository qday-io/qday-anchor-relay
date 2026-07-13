-- 花費是否已被鏈上觀測到（scanner 在某筆交易的 Vin 看到該 coin 的 serial）。
-- ConfirmLost 復活 coin 時必須排除這類 coin：它們的花費是真實的。
ALTER TABLE relay_coins ADD COLUMN spent_seen_on_chain BOOLEAN NOT NULL DEFAULT false;
