-- The clean series, klines_clean, of clean market data 0.1.0 (README.md, The store), whose value carries no read: its
-- column read dropped. The table is empty until a clean topic is made; dropping a column changes the catalog alone,
-- the hypertable's chunks with it, and PostgreSQL rewrites no row.
ALTER TABLE klines_clean DROP COLUMN read;
