-- Massive's own fields, vw and n, each optional, as Massive's spec has its row (o, h, l, c, v and t required, vw and n
-- not): NULL where a bar has none. ClickHouse rewrites the two columns of each part, a map of their NULLs beside them,
-- every value the same; the statement waits for that (mutations_sync), and may run again: a column already of the
-- type is left as it is.
ALTER TABLE klines
    MODIFY COLUMN vw Nullable(Decimal(21, 6)),
    MODIFY COLUMN n  Nullable(Int64)
SETTINGS mutations_sync = 2;
