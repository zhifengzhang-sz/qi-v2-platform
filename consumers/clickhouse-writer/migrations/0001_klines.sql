-- Massive's K-lines: the contract's fields under the contract's names (market 1.0.0, Massive's K-line), and the
-- series' instrument and timeframe; date the bar's begin, milliseconds since 1970 UTC. A bar once by its key when
-- read with FINAL: of the rows of one key, the one inserted last. Each statement may run again (IF NOT EXISTS): a
-- migration's statements and its row in schema_migrations are not one transaction.
CREATE TABLE IF NOT EXISTS klines (
    instrument String,
    timeframe  String,
    `index`    Int64,
    date       Int64,
    open       Decimal(15, 6),
    high       Decimal(15, 6),
    low        Decimal(15, 6),
    close      Decimal(15, 6),
    volume     Decimal(15, 6),
    vw         Decimal(15, 6),
    n          Int64
) ENGINE = ReplacingMergeTree
ORDER BY (instrument, timeframe, date);
