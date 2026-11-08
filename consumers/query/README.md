# The query

A consumer on the core (`../../core/README.md`), a module of its own on the core and market, franz-go, its admin
client and yaml.v3 its other modules: it answers the common questions of the core's two topics, `market.raw` and
`market.corrections`, and of a clean topic, from the stream, knowing their schemas (`core/raw`, `core/clean`) and each
series' zone (the catalog), so that no one needs a partition, an offset or a millisecond to read the record. It reads only, in no group: no topic written,
no group joined or made, no offset committed. It is this folder and its entry in the root `Makefile`'s `CORE`; removing
those, and the lines of `../../README.md` that name it, removes it.

## The program

```
query topics      [-stack stack|test]
query series      [-stack stack|test] [-catalog config/catalog.yaml] [-topic market.raw|market.corrections|CLEAN]
query bars        [-series KEY] [-first N | -last N | -from T -to T] [-topic market.raw|market.corrections|CLEAN]
                  [-stack stack|test] [-catalog config/catalog.yaml]
query minute      -series KEY -at T [-stack stack|test] [-catalog config/catalog.yaml]
query corrections -series KEY [-stack stack|test] [-catalog config/catalog.yaml]
query groups      [-stack stack|test]
```

- **The stack** it reads is `-stack`: the stack's Redpanda, `localhost:19092`, by default, or the test stack's,
  `localhost:29092` (`../../docs/impl/engineering/dev.environment.md`, 3).
- **How it reads.** The topics and each partition's offsets by the core's admin (`stream.Admin`'s `List` and `Spans`
  alone); a partition's records by a client of no group consuming that partition directly, from its start to its end
  as it stood when the read began; the groups by the admin client's reads alone (`ListGroups`, `DescribeGroups`,
  `OffsetFetch`).
- **A series** is its key, `<source>:<instrument>:<timeframe>` (`massive:SPY:M1`), and is read from its instrument's
  partition alone, the one the schema puts it on; `series`, which reads every partition, refuses a series found on
  another. **A clean topic**, `CLEAN` (`market.clean.validation.v0-1-0.massive.SPY.M1`, `../../core/README.md`), is one
  series', read from its one partition, its series its name's: `-series`, not needed, must be it. **Its zone** is its dataset's `session.zone` in the catalog (`-catalog`, `config/catalog.yaml` by default,
  the dataset of its source, instrument and timeframe), UTC where none names one; two datasets naming two zones for it
  are refused.
- **A minute**, `T`, is given in the series' zone (`2026-11-04T09:30`), or with an offset or `Z`
  (`2026-11-04T14:30Z`, `2026-11-04T09:30-05:00`); a time the zone's clock passes twice or never, at a change of its
  offset, is refused without one. Each minute is shown in that form, so a minute shown is a `T` to give. `-from` and
  `-to` are both included; either may be given alone.
- **Each message** is read by the core's readers, raw market data 0.3.0, 0.2.0 or 0.1.0 on `market.raw`
  (`raw.FromRecord`), market corrections 0.1.0 on `market.corrections` (`raw.CorrectionFromRecord`), clean market data
  0.1.0 on a clean topic (`clean.FromRecord`); one that is not stops the run, naming its topic, partition and offset.
  **A copy** is dropped as each schema's delivery says: on `market.raw` and a clean topic a message whose key and bar's
  `date` were taken already, the first taken; on `market.corrections` one whose key, `date` and `read` were. On a clean
  topic, `index` is the clean series'.
- **The output** is a table on stdout, its columns aligned; `minute` and `corrections` begin with a line on what they
  found. A K-line's fields are written as its canonical text writes them; `read` is in UTC, to the millisecond; `-` is
  a field a version has not (the source's own fields of the zero K-line, the `read` of a first version of 0.1.0 or
  0.2.0, which carry none).
- **Its exit**: 0; 2 when no subcommand or one not known is given, or a flag not known, given no value, or its value
  not of its kind (a number), with the usage on stderr; 1 on any other fault, a flag's value refused among them (a stack, a topic, a
  key, a minute, a window), said on stderr.

## Each subcommand, on the stack

Each run on the stack as it holds the record: `market.raw` 389,370 messages, all 0.1.0, `market.corrections` 417, both on
partition 7, SPY's; each output as it came. `topics` and `groups` as run in step 8.7, before step 8.8 made the clean
topic and the group `acquisition` there; the clean topic's runs after its first reset (step 8.8). A read of either topic whole takes a few seconds.

### `query topics`

Every topic the brokers list: each of its partitions, its start and end offsets and its messages, then the topic's
partitions and its messages in all. Below, on the stack, two empty topics of the old platform are left out of its
output, their lines alone.

```
$ query topics
topic               partition  start  end     messages
decisions           0          0      0       0
decisions           1          0      0       0
decisions           all 2                     0
fills               0          0      0       0
fills               1          0      0       0
fills               all 2                     0
market.corrections  0          0      0       0
market.corrections  1          0      0       0
market.corrections  2          0      0       0
market.corrections  3          0      0       0
market.corrections  4          0      0       0
market.corrections  5          0      0       0
market.corrections  6          0      0       0
market.corrections  7          0      417     417
market.corrections  8          0      0       0
market.corrections  9          0      0       0
market.corrections  10         0      0       0
market.corrections  11         0      0       0
market.corrections  12         0      0       0
market.corrections  13         0      0       0
market.corrections  14         0      0       0
market.corrections  15         0      0       0
market.corrections  all 16                    417
market.raw          0          0      0       0
market.raw          1          0      0       0
market.raw          2          0      0       0
market.raw          3          0      0       0
market.raw          4          0      0       0
market.raw          5          0      0       0
market.raw          6          0      0       0
market.raw          7          0      389370  389370
market.raw          8          0      0       0
market.raw          9          0      0       0
market.raw          10         0      0       0
market.raw          11         0      0       0
market.raw          12         0      0       0
market.raw          13         0      0       0
market.raw          14         0      0       0
market.raw          15         0      0       0
market.raw          all 16                    389370
raw-klines          0          0      194490  194490
raw-klines          1          0      0       0
raw-klines          all 2                     194490
```

### `query series`

Each series on a core topic, `market.raw` by default: its messages; its minutes, each `date` once; its copies, the
messages dropped by the topic's delivery; its first and last minute, in its zone. On `market.corrections` the messages
less the copies are the corrections.

```
$ query series
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  389370    194880   194490  2024-11-06T09:30-05:00  2026-11-06T15:59-05:00  America/New_York

$ query series -topic market.corrections
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  417       417      0       2026-11-04T09:30-05:00  2026-11-06T15:42-05:00  America/New_York
```

On a clean topic, its one series, run on the stack after its first reset (2026-11-08T04:41:13.684Z, step 8.8), the
topic's 194,880 K-lines, none refused:

```
$ query series -topic market.clean.validation.v0-1-0.massive.SPY.M1
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  194880    194880   0       2024-11-06T09:30-05:00  2026-11-06T15:59-05:00  America/New_York
```

### `query bars`

The series' K-lines, a line each, in the order of their minutes: the minute in the series' zone and in UTC, `index`,
`open`, `high`, `low`, `close`, `volume`, the source's own fields; on `market.raw` each minute once, its first copy; on
`market.corrections` each correction once, in the order of its minute and then its `read`, with its `read` and the
fields it changed (`corrected`). `-first N` and `-last N` are the first and the last N lines; `-from T` and `-to T`
the minutes from and to `T`; with none, every line.

```
$ query bars -series massive:SPY:M1 -first 2
America/New_York        UTC                index  open    high    low     close   volume   n      vw
2024-11-06T09:30-05:00  2024-11-06T14:30Z  0      589.2   589.43  587.87  587.95  1445628  11270  588.909
2024-11-06T09:31-05:00  2024-11-06T14:31Z  1      587.95  588.33  587.77  588.28  770310   5984   588.0403

$ query bars -series massive:SPY:M1 -last 2
America/New_York        UTC                index   open    high    low     close   volume          n      vw
2026-11-06T15:58-05:00  2026-11-06T20:58Z  194878  735.67  736.02  735.5   735.75  715150.169187   7709   735.2802
2026-11-06T15:59-05:00  2026-11-06T20:59Z  194879  735.74  736.07  735.16  735.34  1733623.916612  16034  735.136

$ query bars -series massive:SPY:M1 -from 2026-11-04T09:30 -to 2026-11-04T09:32
America/New_York        UTC                index   open     high    low     close   volume         n     vw
2026-11-04T09:30-05:00  2026-11-04T14:30Z  193710  762.99   763.8   762.88  763.62  791544.932666  9971  763.2852
2026-11-04T09:31-05:00  2026-11-04T14:31Z  193711  763.595  764.35  763.42  764.07  357872.412584  3975  763.8343
2026-11-04T09:32-05:00  2026-11-04T14:32Z  193712  764.11   764.25  763.36  763.36  202515.974315  2886  763.933

$ query bars -series massive:SPY:M1 -topic market.corrections -first 2
America/New_York        UTC                index   open     high    low     close   volume         n     vw        read                      corrected
2026-11-04T09:30-05:00  2026-11-04T14:30Z  193710  762.99   763.8   762.88  763.62  791544.932666  9971  763.2861  2026-11-08T01:22:29.250Z  vw
2026-11-04T09:31-05:00  2026-11-04T14:31Z  193711  763.595  764.35  763.42  764.07  357872.412584  3975  763.8352  2026-11-08T01:22:29.250Z  vw
```

On a clean topic, `-series` not needed, its index the clean series' (here the grid's, the validation having refused
none); each minute its latest version cleaned when the topic was made, as 2026-11-06 10:00's `low`, 734.64 there,
corrected on `market.corrections`; run on the stack after its first reset (step 8.8):

```
$ query bars -topic market.clean.validation.v0-1-0.massive.SPY.M1 -first 2
America/New_York        UTC                index  open    high    low     close   volume   n      vw
2024-11-06T09:30-05:00  2024-11-06T14:30Z  0      589.2   589.43  587.87  587.95  1445628  11270  588.909
2024-11-06T09:31-05:00  2024-11-06T14:31Z  1      587.95  588.33  587.77  588.28  770310   5984   588.0403

$ query bars -topic market.clean.validation.v0-1-0.massive.SPY.M1 -last 2
America/New_York        UTC                index   open    high    low     close   volume          n      vw
2026-11-06T15:58-05:00  2026-11-06T20:58Z  194878  735.67  736.02  735.5   735.75  715150.169187   7709   735.2802
2026-11-06T15:59-05:00  2026-11-06T20:59Z  194879  735.74  736.07  735.16  735.34  1733623.916612  16034  735.136

$ query bars -topic market.clean.validation.v0-1-0.massive.SPY.M1 -from 2026-11-06T10:00 -to 2026-11-06T10:01
America/New_York        UTC                index   open    high      low     close   volume         n     vw
2026-11-06T10:00-05:00  2026-11-06T15:00Z  194520  740.59  741.4099  734.64  741.05  265563.715347  4123  740.6651
2026-11-06T10:01-05:00  2026-11-06T15:01Z  194521  741.04  741.12    735.12  739.83  183356.071191  3623  740.154
```

### `query minute`

One minute of the series: its first version on `market.raw`, then each of its corrections in the order of their
`read`, each with the fields it changed.

```
$ query minute -series massive:SPY:M1 -at 2026-11-04T09:30
massive:SPY:M1 at 2026-11-04T09:30-05:00 (2026-11-04T14:30Z): its first version on market.raw, 1 copy dropped; 1 correction on market.corrections, 0 copies dropped
version     read                      corrected  index   open    high   low     close   volume         n     vw
first       -                         -          193710  762.99  763.8  762.88  763.62  791544.932666  9971  763.2852
correction  2026-11-08T01:22:29.250Z  vw         193710  762.99  763.8  762.88  763.62  791544.932666  9971  763.2861
```

### `query corrections`

The series' corrections by day, in its zone: each day's corrections, the minutes they correct, how many change each
field, how many change `high` or `low`, and in all; then by day and the fields changed.

```
$ query corrections -series massive:SPY:M1
massive:SPY:M1: 417 corrections on market.corrections, 0 copies dropped, of 417 minutes; 14 changing high or low; the days in America/New_York
day         corrections  minutes  high  low  n   volume  vw   high-or-low
2026-11-04  388          388      0     0    0   0       388  0
2026-11-06  29           29       6     9    29  29      25   14
all         417          417      6     9    29  29      413  14

day         corrected             corrections
2026-11-04  vw                    388
2026-11-06  high,low,n,volume,vw  1
2026-11-06  high,n,volume,vw      5
2026-11-06  low,n,volume          1
2026-11-06  low,n,volume,vw       7
2026-11-06  n,volume              3
2026-11-06  n,volume,vw           12
```

### `query groups`

Each consumer group: for each topic it has committed on or its members are assigned, each partition it has committed
on, is assigned or that holds a message: its committed offset (the next it reads), the partition's end, and how far
behind it is, from its committed offset, or with none (`-`) from the partition's start, where the core's consumers
begin; a group of no topic, a line of `-`.

```
$ query groups
group               state  topic               partition  committed  end     behind
clickhouse-writer   Empty  market.corrections  7          417        417     0
clickhouse-writer   Empty  market.raw          7          389370     389370  0
timescaledb-writer  Empty  market.corrections  7          417        417     0
timescaledb-writer  Empty  market.raw          7          389370     389370  0
```

## The tests

`query_test.go`, against the core's in-memory bus (`core/stream/mem`), each subcommand on one fixture: on `market.raw`
four minutes of a series (one of 0.2.0, one the zero K-line), another series on its partition and one on another, a
copy of one minute and one of another value, each dropped, the first taken; on `market.corrections` one minute
corrected twice and its first correction again, a copy dropped, and the zero K-line corrected to a bar; groups committed whole and in
part. The in-memory bus reads only in a group, so the tests' stand-in reads it in one of its own, which the groups it
lists leave out; on Kafka `query` joins none, tried on the test stack (`-stack test`). Every topic's partitions and ends
and every group's offsets are the same after each subcommand as before the first; the refusals (a flag, a key, a minute
twice or never in the zone, a message not of the schema).
