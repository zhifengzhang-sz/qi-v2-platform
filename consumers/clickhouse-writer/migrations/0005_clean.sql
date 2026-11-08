-- The clean series, klines_clean, of clean market data 0.1.0 (README.md, The store), whose value carries no read: its
-- column read dropped. The table is empty until a clean topic is made; the statement waits for the change
-- (mutations_sync), and may run again: a column not there is left so.
ALTER TABLE klines_clean DROP COLUMN IF EXISTS `read` SETTINGS mutations_sync = 2;
