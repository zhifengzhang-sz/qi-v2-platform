-- Massive's K-lines: the contract's fields under the contract's names (market 1.0.0, Massive's K-line), and the
-- series' instrument and timeframe; date the bar's begin, milliseconds since 1970 UTC; a bar once by its key.
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE klines (
    instrument text           NOT NULL,
    timeframe  text           NOT NULL,
    "index"    bigint         NOT NULL,
    date       bigint         NOT NULL,
    open       numeric(15, 6) NOT NULL,
    high       numeric(15, 6) NOT NULL,
    low        numeric(15, 6) NOT NULL,
    close      numeric(15, 6) NOT NULL,
    volume     numeric(15, 6) NOT NULL,
    vw         numeric(15, 6) NOT NULL,
    n          bigint         NOT NULL,
    PRIMARY KEY (instrument, timeframe, date)
);

-- chunks of 7 days of date, TimescaleDB's default for a time column, in date's unit
SELECT create_hypertable('klines', by_range('date', 604800000));
