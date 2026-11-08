-- The four series (README.md, The store), no row of klines changed. The raw, klines: each minute's first version, from
-- market.raw, now with read, when the run that published it read it, NULL where its value carries none (0.1.0, 0.2.0):
-- a column added changes the table's metadata alone, ClickHouse rewriting no part, its rows reading NULL there. The
-- corrections, from market.corrections, each by its minute and its read, with the fields it names. The corrected raw,
-- each minute's latest version, the one of the greatest read: today the raw, as no correction exists, so its rows are
-- klines' rows, read with FINAL. The clean, empty until a clean topic is made. Each a ReplacingMergeTree read with
-- FINAL, as klines. Each statement may run again: the INSERT, run again, inserts the same rows, which FINAL reads once.
ALTER TABLE klines ADD COLUMN IF NOT EXISTS `read` Nullable(Int64);
CREATE TABLE IF NOT EXISTS corrections (
    instrument String,
    timeframe  String,
    `index`    Int64,
    date       Int64,
    open       Decimal(21, 6),
    high       Decimal(21, 6),
    low        Decimal(21, 6),
    close      Decimal(21, 6),
    volume     Decimal(21, 6),
    vw         Nullable(Decimal(21, 6)),
    n          Nullable(Int64),
    `read`     Int64,
    corrected  Array(String)
) ENGINE = ReplacingMergeTree
ORDER BY (instrument, timeframe, date, `read`);
CREATE TABLE IF NOT EXISTS klines_corrected (
    instrument String,
    timeframe  String,
    `index`    Int64,
    date       Int64,
    open       Decimal(21, 6),
    high       Decimal(21, 6),
    low        Decimal(21, 6),
    close      Decimal(21, 6),
    volume     Decimal(21, 6),
    vw         Nullable(Decimal(21, 6)),
    n          Nullable(Int64),
    `read`     Nullable(Int64)
) ENGINE = ReplacingMergeTree
ORDER BY (instrument, timeframe, date);
INSERT INTO klines_corrected (instrument, timeframe, `index`, date, open, high, low, close, volume, vw, n, `read`)
SELECT instrument, timeframe, `index`, date, open, high, low, close, volume, vw, n, `read` FROM klines FINAL;
CREATE TABLE IF NOT EXISTS klines_clean (
    instrument String,
    timeframe  String,
    `index`    Int64,
    date       Int64,
    open       Decimal(21, 6),
    high       Decimal(21, 6),
    low        Decimal(21, 6),
    close      Decimal(21, 6),
    volume     Decimal(21, 6),
    vw         Nullable(Decimal(21, 6)),
    n          Nullable(Int64),
    `read`     Nullable(Int64)
) ENGINE = ReplacingMergeTree
ORDER BY (instrument, timeframe, date);
