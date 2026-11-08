-- The four series (README.md, The store), no row of klines changed. The raw, klines: each minute's first version, from
-- market.raw, now with read, when the run that published it read it, NULL where its value carries none (0.1.0, 0.2.0):
-- a column added that may be NULL, without a default, changes the catalog alone, the hypertable's chunks with it, and
-- PostgreSQL rewrites no row. The corrections, from market.corrections, each by its minute and its read, with the
-- fields it names. The corrected raw, each minute's latest version, the one of the greatest read: today the raw, as no
-- correction exists, so its rows are klines' rows. The clean, empty until a clean topic is made. Each a hypertable on
-- date, chunks of 7 days of it, as klines.
ALTER TABLE klines ADD COLUMN read bigint;

CREATE TABLE corrections (
    instrument text           NOT NULL,
    timeframe  text           NOT NULL,
    "index"    bigint         NOT NULL,
    date       bigint         NOT NULL,
    open       numeric(21, 6) NOT NULL,
    high       numeric(21, 6) NOT NULL,
    low        numeric(21, 6) NOT NULL,
    close      numeric(21, 6) NOT NULL,
    volume     numeric(21, 6) NOT NULL,
    vw         numeric(21, 6),
    n          bigint,
    read       bigint         NOT NULL,
    corrected  text[]         NOT NULL,
    PRIMARY KEY (instrument, timeframe, date, read)
);
SELECT create_hypertable('corrections', by_range('date', 604800000));

CREATE TABLE klines_corrected (
    instrument text           NOT NULL,
    timeframe  text           NOT NULL,
    "index"    bigint         NOT NULL,
    date       bigint         NOT NULL,
    open       numeric(21, 6) NOT NULL,
    high       numeric(21, 6) NOT NULL,
    low        numeric(21, 6) NOT NULL,
    close      numeric(21, 6) NOT NULL,
    volume     numeric(21, 6) NOT NULL,
    vw         numeric(21, 6),
    n          bigint,
    read       bigint,
    PRIMARY KEY (instrument, timeframe, date)
);
SELECT create_hypertable('klines_corrected', by_range('date', 604800000));
INSERT INTO klines_corrected (instrument, timeframe, "index", date, open, high, low, close, volume, vw, n, read)
SELECT instrument, timeframe, "index", date, open, high, low, close, volume, vw, n, read FROM klines;

CREATE TABLE klines_clean (
    instrument text           NOT NULL,
    timeframe  text           NOT NULL,
    "index"    bigint         NOT NULL,
    date       bigint         NOT NULL,
    open       numeric(21, 6) NOT NULL,
    high       numeric(21, 6) NOT NULL,
    low        numeric(21, 6) NOT NULL,
    close      numeric(21, 6) NOT NULL,
    volume     numeric(21, 6) NOT NULL,
    vw         numeric(21, 6),
    n          bigint,
    read       bigint,
    PRIMARY KEY (instrument, timeframe, date)
);
SELECT create_hypertable('klines_clean', by_range('date', 604800000));
