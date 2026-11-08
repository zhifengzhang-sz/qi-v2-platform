# Basic: reading the platform

The test stack filled from Massive, the core's topics read with `query`, then a reader of your own reading the clean
topic from its start to its end. What you need, and how the commands and outputs are given: `../README.md`.

## 1. The test stack, filled from Massive

A folder of your own for the tutorial, a temporary one: Massive's pages go there, never into the stack's
(`~/.local/share/qi/massive/pages`), and so do the files the readers write.

```
$ export QI_TUTORIAL=$(mktemp -d)
$ echo $QI_TUTORIAL
/tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp
```

The tutorial's catalog, `catalog.yaml` beside this page, is `massive-spy-1m` as `config/catalog.yaml` has it, SPY's
minutes of the regular session from Massive, two years of history, but where its reader keeps the pages. Your copy of
it names the repository's root and your folder:

```
$ sed -e "s|@ROOT@|$PWD|g" -e "s|@PAGES@|$QI_TUTORIAL/pages|" docs/tutorial/basic/catalog.yaml > $QI_TUTORIAL/catalog.yaml
```

Then one command brings the test stack up, builds the core's programs into `bin/`, makes the core's topics, and starts
the platform's components on it, each in the background: acquisition, which reads Massive through the catalog's
reader and publishes every minute, and the two stores' writers (`../../impl/engineering/dev.environment.md`, 3):

```
$ make start STACK=test CATALOG=$QI_TUTORIAL/catalog.yaml
make stack-test-up
make[1]: Entering directory '/home/zzhang/dev/qi/github/qi-v2-platform'
nix run --inputs-from . services#stack-test -- up -d --wait
warning: Git tree '/home/zzhang/dev/qi/github/qi-v2-platform' is dirty
 Network services-test_qitest-network Creating 
 Volume services-test_timescaledb_data Creating 
 Volume services-test_timescaledb_data Creating 
 Network services-test_qitest-network Creating 
 Volume services-test_clickhouse_data Creating 
 Volume services-test_clickhouse_data Creating 
 Volume services-test_redpanda_data Creating 
 Volume services-test_redpanda_data Creating 
 Volume services-test_timescaledb_data Created 
 Volume services-test_timescaledb_data Created 
 Volume services-test_clickhouse_data Created 
 Volume services-test_clickhouse_data Created 
 Volume services-test_redpanda_data Created 
 Volume services-test_redpanda_data Created 
 Network services-test_qitest-network Created 
 Network services-test_qitest-network Created 
 Container qitest-timescaledb Creating 
 Container qitest-clickhouse Creating 
 Container qitest-redpanda Creating 
 Container qitest-timescaledb Created 
 Container qitest-clickhouse Created 
 Container qitest-redpanda Created 
 Container qitest-console Creating 
 Container qitest-console Created 
 Container qitest-timescaledb Starting 
 Container qitest-redpanda Starting 
 Container qitest-clickhouse Starting 
 Container qitest-timescaledb Started 
 Container qitest-redpanda Started 
 Container qitest-redpanda Waiting 
 Container qitest-clickhouse Started 
 Container qitest-redpanda Healthy 
 Container qitest-console Starting 
 Container qitest-console Started 
 Container qitest-clickhouse Waiting 
 Container qitest-console Waiting 
 Container qitest-redpanda Waiting 
 Container qitest-timescaledb Waiting 
 Container qitest-redpanda Healthy 
 Container qitest-timescaledb Healthy 
 Container qitest-clickhouse Healthy 
 Container qitest-console Healthy 
make[1]: Leaving directory '/home/zzhang/dev/qi/github/qi-v2-platform'
GOPROXY=off go build -o bin/ ./core/cmd/provision ./publishers/acquisition ./consumers/printer ./consumers/timescaledb-writer ./consumers/clickhouse-writer ./consumers/query ./plugins/massive-reader ./plugins/validation
CATALOG="/tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/catalog.yaml" DATASET="" python3 dev/scripts/run.py start test
market.raw 16 partitions, retention unlimited, timestamps CreateTime, created
market.corrections 16 partitions, retention unlimited, timestamps CreateTime, created
acquisition: started, pid 621270, log /home/zzhang/.local/state/qi/run/test/acquisition.log
timescaledb-writer: its group's committed offsets: none; it takes up there, or at the topics' start when its store holds no bar (its log's from_start)
timescaledb-writer: started, pid 621378, log /home/zzhang/.local/state/qi/run/test/timescaledb-writer.log
clickhouse-writer: its group's committed offsets: none; it takes up there, or at the topics' start when its store holds no bar (its log's from_start)
clickhouse-writer: started, pid 621436, log /home/zzhang/.local/state/qi/run/test/clickhouse-writer.log
```

Acquisition runs at once: it asks Massive, with your key, for every minute from the history's first to the last
minute that is 20 h old (the catalog's `delay`), publishes them, then cleans the series and makes its clean topic. Its
log says what it does; the run is done at its line on the next run (Ctrl-C ends the watch, not acquisition):

```
$ tail -f ~/.local/state/qi/run/test/acquisition.log
== start 2026-11-08T09:41:36.433Z on the test: /home/zzhang/dev/qi/github/qi-v2-platform/bin/acquisition -dataset massive-spy-1m -catalog /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/catalog.yaml -brokers localhost:29092 -corrected /home/zzhang/dev/qi/github/qi-v2-platform/bin/timescaledb-writer -brokers localhost:29092 -url postgres://postgres:password@localhost:15432/postgres -store /home/zzhang/dev/qi/github/qi-v2-platform/bin/timescaledb-writer -brokers localhost:29092 -url postgres://postgres:password@localhost:15432/postgres -store /home/zzhang/dev/qi/github/qi-v2-platform/bin/clickhouse-writer -brokers localhost:29092 -url http://default:password@localhost:18123 -follow
massive-reader: SPY:M1, 500 spans in [1731076200000, 1793998740000]: 432088 rows given by Massive in 9 pages; 194100 printed, 0 not closed, 237988 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 9 saved, 0 kept already
acquisition: massive:SPY:M1: the record: none on market.raw; asked 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 194100 minutes of the grid in 500 spans; 194100 lines; published 194100 first versions on market.raw (index 0 to 194099; 0 zero K-lines), 0 corrections on market.corrections; 0 unchanged
time=2026-11-08T17:43:21.227+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 74000 of 194100"
time=2026-11-08T17:43:22.236+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 74000 of 194100"
time=2026-11-08T17:43:23.244+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 92000 of 194100"
time=2026-11-08T17:43:24.255+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 110000 of 194100"
time=2026-11-08T17:43:25.265+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 110000 of 194100"
time=2026-11-08T17:43:26.274+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 128000 of 194100"
time=2026-11-08T17:43:27.282+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 144000 of 194100"
time=2026-11-08T17:43:28.291+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 144000 of 194100"
time=2026-11-08T17:43:29.299+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 160000 of 194100"
time=2026-11-08T17:43:30.311+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 160000 of 194100"
time=2026-11-08T17:43:31.318+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 176000 of 194100"
time=2026-11-08T17:43:32.325+08:00 level=INFO msg="waiting for the group at the topics' ends" group=timescaledb-writer behind="market.raw/7 at 192000 of 194100"
timescaledb-writer: -corrected massive:SPY:M1: 194100 minutes of klines_corrected
timescaledb-writer: -clean market.clean.validation.v0-1-0.massive.SPY.M1: 194100 K-lines read, 0 copies dropped (0 of another value); the clean series: 194100 rows written, the series emptied first
clickhouse-writer: -clean market.clean.validation.v0-1-0.massive.SPY.M1: 194100 K-lines read, 0 copies dropped (0 of another value); the clean series: 194100 rows written, the series emptied first
acquisition: massive:SPY:M1: the clean: market.clean.validation.v0-1-0.massive.SPY.M1, by validation 0.1.0: reset at 2026-11-08T09:43:21.187Z, its first reset, no clean topic before; the topics derived from it deleted: none; made again from timescaledb-writer's corrected raw, 194100 minutes: 194100 published, 0 refused; the stores' clean series written from it: timescaledb-writer, clickhouse-writer; now 194100 K-lines, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST; the damage after it 0
acquisition: the next run at 2026-11-10 12:00 EST, when the last bar of the next session is 20h old
^C
```

Massive gave its rows in pages, each saved once in your folder; acquisition published one first version a minute, its
`index` its place in the series' grid, then made the clean topic by its first reset. Acquisition keeps running, its
next run when the next session's last minute is 20 h old; the writers keep both stores.

## 2. The core's topics, read with `query`

`query` (`../../../consumers/query/README.md`) answers the common questions of the core's topics, read only, in no
group, with `-stack test` the test stack's.

```
$ bin/query topics -stack test
topic                                          partition  start  end     messages
market.clean.validation.v0-1-0.massive.SPY.M1  0          0      194100  194100
market.clean.validation.v0-1-0.massive.SPY.M1  all 1                     194100
market.corrections                             0          0      0       0
market.corrections                             1          0      0       0
market.corrections                             2          0      0       0
market.corrections                             3          0      0       0
market.corrections                             4          0      0       0
market.corrections                             5          0      0       0
market.corrections                             6          0      0       0
market.corrections                             7          0      0       0
market.corrections                             8          0      0       0
market.corrections                             9          0      0       0
market.corrections                             10         0      0       0
market.corrections                             11         0      0       0
market.corrections                             12         0      0       0
market.corrections                             13         0      0       0
market.corrections                             14         0      0       0
market.corrections                             15         0      0       0
market.corrections                             all 16                    0
market.raw                                     0          0      0       0
market.raw                                     1          0      0       0
market.raw                                     2          0      0       0
market.raw                                     3          0      0       0
market.raw                                     4          0      0       0
market.raw                                     5          0      0       0
market.raw                                     6          0      0       0
market.raw                                     7          0      194100  194100
market.raw                                     8          0      0       0
market.raw                                     9          0      0       0
market.raw                                     10         0      0       0
market.raw                                     11         0      0       0
market.raw                                     12         0      0       0
market.raw                                     13         0      0       0
market.raw                                     14         0      0       0
market.raw                                     15         0      0       0
market.raw                                     all 16                    194100
```

- **`market.raw`** holds each minute's first version, as the run that published it read it, for every source and
  instrument, keyed `<source>:<instrument>:<timeframe>`, 16 partitions, a series on its instrument's (SPY's is 7). It
  keeps every record, forever: it is the raw record, from whose start anything can be made again.
- **`market.corrections`** holds each later version of a minute that differs from the one before it, as a later run
  read it again, and the fields that differ; keyed and partitioned as `market.raw`, kept forever.
- **`market.clean.validation.v0-1-0.massive.SPY.M1`** is the clean topic of one series, `massive:SPY:M1`, by one
  cleaning, the validation 0.1.0: each K-line it gave, in the order of their dates; one partition.

```
$ bin/query series -stack test
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  194100    194100   0       2024-11-08T09:30-05:00  2026-11-06T15:59-05:00  America/New_York
$ bin/query series -stack test -topic market.corrections
series  messages  minutes  copies  first  last  zone
$ bin/query series -stack test -topic market.clean.validation.v0-1-0.massive.SPY.M1
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  194100    194100   0       2024-11-08T09:30-05:00  2026-11-06T15:59-05:00  America/New_York
```

A series' K-lines, a line each: the minute in the series' zone and in UTC, then the K-line's fields as its canonical
text writes them. `index` is the minute's place in the series' grid, every minute of the session from the series'
first, and never moves; `date` is the minute's begin, milliseconds since 1970 in the value, shown as a time here;
`open`, `high`, `low`, `close` and `volume` are market 1.1.0's base K-line; `n` and `vw` are Massive's own fields
(trades and the volume-weighted price), read by Massive's reader.

```
$ bin/query bars -stack test -series massive:SPY:M1 -first 3
America/New_York        UTC                index  open     high    low      close   volume  n     vw
2024-11-08T09:30-05:00  2024-11-08T14:30Z  0      596.17   596.61  596.165  596.56  500390  4161  596.3504
2024-11-08T09:31-05:00  2024-11-08T14:31Z  1      596.56   596.59  596.39   596.53  205144  1987  596.4922
2024-11-08T09:32-05:00  2024-11-08T14:32Z  2      596.535  596.62  596.515  596.56  101060  1335  596.5715
$ bin/query bars -stack test -series massive:SPY:M1 -last 2
America/New_York        UTC                index   open    high    low     close   volume          n      vw
2026-11-06T15:58-05:00  2026-11-06T20:58Z  194098  735.67  736.02  735.5   735.75  715150.169187   7709   735.2802
2026-11-06T15:59-05:00  2026-11-06T20:59Z  194099  735.74  736.07  735.16  735.34  1733623.916612  16034  735.136
```

**The zero K-line.** A minute of the grid that Massive gives no bar for, a minute without an eligible trade, is on
`market.raw` all the same, at its place: the zero K-line, `open`, `high`, `low`, `close` and `volume` 0, no field of the
source's own. SPY trades every minute of its session, so this series holds none: the run's line above says `0 zero
K-lines`, and the minutes of `query series` are the grid's.

**A correction.** Massive revises a minute after it has given it, and says so nowhere; each run of acquisition asks
again for the last 30 days (the catalog's `reread`), and a minute whose K-line now differs from its latest version is
published on `market.corrections`, naming the fields that differ. A fresh test stack has had one run, so none yet; the
advanced level writes some. A minute's every version:

```
$ bin/query corrections -stack test -series massive:SPY:M1
massive:SPY:M1: 0 corrections on market.corrections, 0 copies dropped, of 0 minutes; 0 changing high or low; the days in America/New_York
$ bin/query minute -stack test -series massive:SPY:M1 -at 2026-11-06T15:59
massive:SPY:M1 at 2026-11-06T15:59-05:00 (2026-11-06T20:59Z): its first version on market.raw, 0 copies dropped; 0 corrections on market.corrections, 0 copies dropped
version  read                      corrected  index   open    high    low     close   volume          n      vw
first    2026-11-08T09:43:14.885Z  -          194099  735.74  736.07  735.16  735.34  1733623.916612  16034  735.136
```

**The clean topic and its index.** A cleaning judges each minute's latest version and gives it or refuses it, with its
reason (`../../impl/engineering/plugins.md`, 3); the clean topic holds what it gave, its `index` its place in the clean
series from 0, which is the grid's here only because the validation refused none. When a correction changes a `high`
or a `low` of more minutes than the dataset's `tolerance` (10 here), acquisition makes the topic again under its name
(the advanced level).

```
$ bin/query bars -stack test -topic market.clean.validation.v0-1-0.massive.SPY.M1 -last 2
America/New_York        UTC                index   open    high    low     close   volume          n      vw
2026-11-06T15:58-05:00  2026-11-06T20:58Z  194098  735.67  736.02  735.5   735.75  715150.169187   7709   735.2802
2026-11-06T15:59-05:00  2026-11-06T20:59Z  194099  735.74  736.07  735.16  735.34  1733623.916612  16034  735.136
```

Where each consumer group stands, the lines of a partition it has committed on: both stores' writers, each at the
end of `market.raw`'s partition 7, SPY's; `market.corrections` holds nothing yet, so neither has committed on it.
Acquisition reads the record in a group of its own and commits nothing, and `query` joins none.

```
$ bin/query groups -stack test | awk 'NR == 1 || $5 != "-"'
group               state   topic               partition  committed  end     behind
clickhouse-writer   Stable  market.raw          7          194100     194100  0
timescaledb-writer  Stable  market.raw          7          194100     194100  0
```

## 3. A reader of your own

`clean-reader/main.go` reads the clean topic from its start to its end, in no group, and prints one line a K-line,
its index, date (UTC), open, high, low, close and volume, then a count on stderr. It imports franz-go and market's Go
form alone, no package of this repository, as a reader outside it would: it relies on the platform contract
(`../../../README.md`, The platform contract) and on the contracts, `core/README.md`'s clean market data 0.1.0 and
market 1.1.0's K-line (here it builds in the repository's workspace, which gives market's Go form, the flake's). How
it reads:

- the topic's one partition, 0, by a client of no group (franz-go's `ConsumePartitions`), from its start, to its end
  as it stood when the read began (the admin client's `ListEndOffsets`);
- each value as clean market data 0.1.0 states it, else it stops: one JSON object of its fields alone, `schema`
  `market.clean` and `version` `0.1.0`, its `cleaning`'s name and version, its `origin` and its names well formed (its
  series' three holding no dot), the record's topic the one its cleaning and series name, its key the series of its
  `source`, `instrument` and `timeframe`, the record's timestamp its K-line's `date`, and the value in its canonical
  text (its fields in the order of their names, no spaces, each number as market writes it);
- its `kline` read by market's means: `index` and `date` whole numbers, each price and the volume a decimal read
  exactly from its digits (`market.ParseDecimal`), never a float, and the K-line's rules checked (`market.KLine`'s
  `Check`); each other field the source's, a name and a number in the text market writes for it, its meaning (Massive's
  `n` and `vw`) left to Massive's reader;
- a copy, a value of a key and a date it has taken already, dropped, the first taken, as the schema's delivery says.

```
$ go build -o bin/ ./docs/tutorial/basic/clean-reader
$ bin/clean-reader -topic market.clean.validation.v0-1-0.massive.SPY.M1 > $QI_TUTORIAL/clean.txt
clean-reader: market.clean.validation.v0-1-0.massive.SPY.M1: 194100 messages, 194100 K-lines, 0 copies dropped
$ head -3 $QI_TUTORIAL/clean.txt
0 2024-11-08T14:30Z 596.17 596.61 596.165 596.56 500390
1 2024-11-08T14:31Z 596.56 596.59 596.39 596.53 205144
2 2024-11-08T14:32Z 596.535 596.62 596.515 596.56 101060
$ tail -2 $QI_TUTORIAL/clean.txt
194098 2026-11-06T20:58Z 735.67 736.02 735.5 735.75 715150.169187
194099 2026-11-06T20:59Z 735.74 736.07 735.16 735.34 1733623.916612
```

Its K-lines are, field for field, those `query` gives of the topic (its UTC, `index`, `open`, `high`, `low`, `close`
and `volume` columns, in the reader's order):

```
$ bin/query bars -stack test -topic market.clean.validation.v0-1-0.massive.SPY.M1 | awk 'NR > 1 {print $3, $2, $4, $5, $6, $7, $8}' | md5sum
1d0b7a6b60450d2c774ee535d0bc04d9  -
$ md5sum < $QI_TUTORIAL/clean.txt
1d0b7a6b60450d2c774ee535d0bc04d9  -
```

The test stack stays up for the next level, `../medium/README.md`. To stop here instead: `make stack-test-down`, which
stops the components and wipes the test stack, and `rm -r $QI_TUTORIAL`.
