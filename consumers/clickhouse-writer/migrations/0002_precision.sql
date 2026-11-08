-- The prices and the volume of the contract's precision: market's decimal has at most 6 digits after the point and 15
-- from the first that is not 0, so up to 15 before the point, which Decimal(21, 6) holds. ClickHouse rewrites the
-- columns of each part, every value the same; the statement waits for that (mutations_sync), and may run again: a
-- column already of the type is left as it is.
ALTER TABLE klines
    MODIFY COLUMN open   Decimal(21, 6),
    MODIFY COLUMN high   Decimal(21, 6),
    MODIFY COLUMN low    Decimal(21, 6),
    MODIFY COLUMN close  Decimal(21, 6),
    MODIFY COLUMN volume Decimal(21, 6),
    MODIFY COLUMN vw     Decimal(21, 6)
SETTINGS mutations_sync = 2;
