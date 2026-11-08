-- Massive's own fields, vw and n, each optional, as Massive's spec has its row (o, h, l, c, v and t required, vw and n
-- not): NULL where a bar has none. Dropping NOT NULL changes the catalog alone, the hypertable's chunks with it:
-- PostgreSQL rewrites no row.
ALTER TABLE klines
    ALTER COLUMN vw DROP NOT NULL,
    ALTER COLUMN n DROP NOT NULL;
