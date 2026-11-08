-- The prices and the volume of the contract's precision: market's decimal has at most 6 digits after the point and 15
-- from the first that is not 0, so up to 15 before the point, which numeric(21, 6) holds. The scale is the same and
-- only the precision grows: PostgreSQL changes the columns' type, the hypertable's chunks with it, without rewriting
-- a row.
ALTER TABLE klines
    ALTER COLUMN open   TYPE numeric(21, 6),
    ALTER COLUMN high   TYPE numeric(21, 6),
    ALTER COLUMN low    TYPE numeric(21, 6),
    ALTER COLUMN close  TYPE numeric(21, 6),
    ALTER COLUMN volume TYPE numeric(21, 6),
    ALTER COLUMN vw     TYPE numeric(21, 6);
