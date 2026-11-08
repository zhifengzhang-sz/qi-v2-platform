# Advanced: a cleaning of your own, and the reset

A cleaning of your own, run by acquisition, its clean topic damaged by corrections of your own and made again; then the
reset as it stands today, with a reader of the topic running. On the test stack the medium level left up
(`../medium/README.md`), with `QI_TUTORIAL` set as the basic level set it.

## 1. The cleaning, tried by hand

A cleaning is a program acquisition runs by a line protocol (`../../impl/engineering/plugins.md`, 3): `--clean
[--after DATE]`; its first line its name and version, `{"cleaning":NAME,"version":VERSION}`, by which its clean topic is
named; then K-lines on stdin, one a line, a series' in the order of their dates, each the source's K-line in its
canonical text, and exactly one answer a K-line, in order, `{"kline":K}`, the K-line it gives, or `{"refused":REASON}`;
exit 0 at the end of its input. `--after` is the date of the last K-line the clean topic holds, for a cleaning that
judges a series in order.

`despike/main.go` is one, its name `despike` and its version 0.1.0, on market's Go form and the standard library
alone. Its rules, each refusal with its reason: the zero K-line, a minute without a bar, is refused; a spike, a K-line
whose range, `high − low`, is more than 2% of its `low`, is refused, the range reckoned exactly, never in floats. Each
K-line is judged alone, so it takes `--after` and needs it not. `correct-high` is the test's tool for section 3.

```
$ go build -o bin/ ./docs/tutorial/advanced/despike ./docs/tutorial/advanced/correct-high
```

Three lines in, a series' in the order of their dates, `--after` the minute before the first: SPY's minute of
2025-04-09 13:19 (New York), whose range was 3.3% of its low; SPY's of 2026-09-15 10:00; and a zero K-line, as
acquisition gives a minute without a bar, made up for the trial at 10:01, since SPY has a bar every minute:

```
$ printf '%s\n' '{"close":512.8,"date":1744219140000,"high":515.05,"index":39649,"low":498.67,"n":57304,"open":499.76,"volume":5250916,"vw":506.7755}' '{"close":759.225,"date":1789480800000,"high":759.5406,"index":178920,"low":759.21,"n":1282,"open":759.5406,"volume":52684.86967,"vw":759.3544}' '{"close":0,"date":1789480860000,"high":0,"index":178921,"low":0,"open":0,"volume":0}' | bin/despike --clean --after 1744219080000
{"cleaning":"despike","version":"0.1.0"}
{"refused":"a spike: range 16.38 (high 515.05 - low 498.67) more than 2% of low"}
{"kline":{"close":759.225,"date":1789480800000,"high":759.5406,"index":178920,"low":759.21,"n":1282,"open":759.5406,"volume":52684.86967,"vw":759.3544}}
{"refused":"the zero K-line: no bar this minute"}
```

## 2. Its clean topic, made by acquisition's first reset

The catalog of this level, `catalog.yaml` beside this page, is the basic level's but its `cleaning`, `despike`, and
its `tolerance`, 3 damaged minutes. Your copy, Massive's pages in the same folder:

```
$ sed -e "s|@ROOT@|$PWD|g" -e "s|@PAGES@|$QI_TUTORIAL/pages|" docs/tutorial/advanced/catalog.yaml > $QI_TUTORIAL/advanced.yaml
```

On this level acquisition is run by hand, a run each time, so the one `make start` keeps running is stopped first; the
stores' writers keep running, since a reset reads the TimescaleDB store's corrected raw (`-corrected`) and writes
each store's clean series (`-store`). The writers' commands are given once, as shell variables:

```
$ kill $(cat ~/.local/state/qi/run/test/acquisition.pid)
$ TS="bin/timescaledb-writer -brokers localhost:29092 -url postgres://postgres:password@localhost:15432/postgres"
$ CH="bin/clickhouse-writer -brokers localhost:29092 -url http://default:password@localhost:18123"
```

A run: it asks Massive again for the last 30 days, here finding nothing changed, then cleans the series by
`despike`. The series has no clean topic of `despike` yet, so the run makes it, its first reset:
`market.clean.despike.v0-1-0.massive.SPY.M1`, named by the cleaning's first line, every minute's latest version through
the cleaning, each K-line it gave published, each refusal on the run's line with its minute and its reason.

```
$ bin/acquisition -dataset massive-spy-1m -catalog $QI_TUTORIAL/advanced.yaml -brokers localhost:29092 -corrected "$TS" -store "$TS" -store "$CH"
massive-reader: SPY:M1, 21 spans in [1791552600000, 1793998740000]: 18623 rows given by Massive in 1 pages; 8190 printed, 0 not closed, 10433 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 1 saved, 0 kept already
acquisition: massive:SPY:M1: the record: 194100 minutes on market.raw, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 0 copies (0 of another value); 0 corrections on market.corrections, 0 copies (0 of another value); asked 2026-10-09 09:30 EDT to 2026-11-06 15:59 EST, 8190 minutes of the grid in 21 spans; 8190 lines; published 0 first versions on market.raw (index none; 0 zero K-lines), 0 corrections on market.corrections; 8190 unchanged
timescaledb-writer: -corrected massive:SPY:M1: 194100 minutes of klines_corrected
timescaledb-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
clickhouse-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
acquisition: massive:SPY:M1: the clean: market.clean.despike.v0-1-0.massive.SPY.M1, by despike 0.1.0: reset at 2026-11-08T09:44:46.594Z, its first reset, no clean topic before; the topics derived from it deleted: none; made again from timescaledb-writer's corrected raw, 194100 minutes: 194092 published, 8 refused (2025-04-07 13:59 EDT: a spike: range 11.0949 (high 509.36 - low 498.2651) more than 2% of low; 2025-04-09 13:19 EDT: a spike: range 16.38 (high 515.05 - low 498.67) more than 2% of low; 2025-04-09 13:20 EDT: a spike: range 14.99 (high 522.29 - low 507.3) more than 2% of low; 2025-04-16 14:31 EDT: a spike: range 12.3154 (high 537.8854 - low 525.57) more than 2% of low; 2025-04-23 12:22 EDT: a spike: range 10.91 (high 545.1 - low 534.19) more than 2% of low; 2025-04-23 12:25 EDT: a spike: range 10.825 (high 545.095 - low 534.27) more than 2% of low; 2025-05-27 13:13 EDT: a spike: range 11.68 (high 590.11 - low 578.43) more than 2% of low; 2026-06-26 15:59 EDT: a spike: range 16.825 (high 733.405 - low 716.58) more than 2% of low); the stores' clean series written from it: timescaledb-writer, clickhouse-writer; now 194092 K-lines, index 0 to 194091, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST; the damage after it 0
$ bin/query series -stack test -topic market.clean.despike.v0-1-0.massive.SPY.M1
series          messages  minutes  copies  first                   last                    zone
massive:SPY:M1  194092    194092   0       2024-11-08T09:30-05:00  2026-11-06T15:59-05:00  America/New_York
```

Its index is its place in the clean series: past a refused minute it is no longer the grid's (2026-09-15 10:00, 178920
in the grid, is 178912 here, below).

## 3. Corrections of your own, the tolerance and the reset

`correct-high` writes corrections on the test stack's `market.corrections`, each a minute's latest version with its
`high` 0.01 more, as market corrections 0.1.0; it refuses any broker but the test stack's, since on the stack
`market.corrections` is acquisition's alone and keeps every message forever:

```
$ bin/correct-high -brokers localhost:19092 -at 2026-09-15T10:00-04:00
correct-high: -brokers localhost:19092: only the test stack's, localhost:29092: on the stack, market.corrections is acquisition's alone and keeps every message forever
```

Its minutes are older than the 30 days each run asks again: inside them, the next run would find Massive's value
differing from the correction and correct the minute back. Three corrections, the tolerance's count:

```
$ bin/correct-high -at 2026-09-15T10:00-04:00 -minutes 3
massive:SPY:M1 at 2026-09-15T10:00-04:00: high 759.5406, now 759.5506, corrected [high]; read 2026-11-08T09:45:19.504Z, on market.corrections
massive:SPY:M1 at 2026-09-15T10:01-04:00: high 759.25, now 759.26, corrected [high]; read 2026-11-08T09:45:19.504Z, on market.corrections
massive:SPY:M1 at 2026-09-15T10:02-04:00: high 759.19, now 759.2, corrected [high]; read 2026-11-08T09:45:19.504Z, on market.corrections
$ bin/query corrections -stack test -series massive:SPY:M1
massive:SPY:M1: 3 corrections on market.corrections, 0 copies dropped, of 3 minutes; 3 changing high or low; the days in America/New_York
day         corrections  minutes  high  high-or-low
2026-09-15  3            3        3     3
all         3            3        3     3

day         corrected  corrections
2026-09-15  high       3
```

The damage is the minutes whose `high` or `low` in the clean topic differs from the cleaned latest version, or that
one holds and the other not; up to the tolerance the clean topic is kept as it is:

```
$ bin/acquisition -dataset massive-spy-1m -catalog $QI_TUTORIAL/advanced.yaml -brokers localhost:29092 -corrected "$TS" -store "$TS" -store "$CH"
massive-reader: SPY:M1, 21 spans in [1791552600000, 1793998740000]: 18623 rows given by Massive in 1 pages; 8190 printed, 0 not closed, 10433 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 0 saved, 1 kept already
acquisition: massive:SPY:M1: the record: 194100 minutes on market.raw, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 0 copies (0 of another value); 3 corrections on market.corrections, 0 copies (0 of another value); asked 2026-10-09 09:30 EDT to 2026-11-06 15:59 EST, 8190 minutes of the grid in 21 spans; 8190 lines; published 0 first versions on market.raw (index none; 0 zero K-lines), 0 corrections on market.corrections; 8190 unchanged
acquisition: massive:SPY:M1: the clean: market.clean.despike.v0-1-0.massive.SPY.M1, by despike 0.1.0: 194092 K-lines before the run (0 copies); the tail: no minute after its last; now 194092 K-lines, index 0 to 194091, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST; the latest versions cleaned, 8 refused; 3 minutes damaged, the tolerance 3: 2026-09-15 10:00 EDT, 2026-09-15 10:01 EDT, 2026-09-15 10:02 EDT
```

One more, past the tolerance: the run makes the topic again under its name, from the TimescaleDB store's corrected raw
checked against the record, each store's clean series written from it, the damage after it 0.

```
$ bin/correct-high -at 2026-09-15T10:03-04:00
massive:SPY:M1 at 2026-09-15T10:03-04:00: high 759.11, now 759.12, corrected [high]; read 2026-11-08T09:45:36.101Z, on market.corrections
$ bin/acquisition -dataset massive-spy-1m -catalog $QI_TUTORIAL/advanced.yaml -brokers localhost:29092 -corrected "$TS" -store "$TS" -store "$CH"
massive-reader: SPY:M1, 21 spans in [1791552600000, 1793998740000]: 18623 rows given by Massive in 1 pages; 8190 printed, 0 not closed, 10433 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 0 saved, 1 kept already
acquisition: massive:SPY:M1: the record: 194100 minutes on market.raw, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 0 copies (0 of another value); 4 corrections on market.corrections, 0 copies (0 of another value); asked 2026-10-09 09:30 EDT to 2026-11-06 15:59 EST, 8190 minutes of the grid in 21 spans; 8190 lines; published 0 first versions on market.raw (index none; 0 zero K-lines), 0 corrections on market.corrections; 8190 unchanged
timescaledb-writer: -corrected massive:SPY:M1: 194100 minutes of klines_corrected
timescaledb-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
clickhouse-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
acquisition: massive:SPY:M1: the clean: market.clean.despike.v0-1-0.massive.SPY.M1, by despike 0.1.0: reset at 2026-11-08T09:45:50.375Z, 4 minutes damaged, more than the tolerance 3: 2026-09-15 10:00 EDT, 2026-09-15 10:01 EDT, 2026-09-15 10:02 EDT, 2026-09-15 10:03 EDT; the topics derived from it deleted: none; made again from timescaledb-writer's corrected raw, 194100 minutes: 194092 published, 8 refused (2025-04-07 13:59 EDT: a spike: range 11.0949 (high 509.36 - low 498.2651) more than 2% of low; 2025-04-09 13:19 EDT: a spike: range 16.38 (high 515.05 - low 498.67) more than 2% of low; 2025-04-09 13:20 EDT: a spike: range 14.99 (high 522.29 - low 507.3) more than 2% of low; 2025-04-16 14:31 EDT: a spike: range 12.3154 (high 537.8854 - low 525.57) more than 2% of low; 2025-04-23 12:22 EDT: a spike: range 10.91 (high 545.1 - low 534.19) more than 2% of low; 2025-04-23 12:25 EDT: a spike: range 10.825 (high 545.095 - low 534.27) more than 2% of low; 2025-05-27 13:13 EDT: a spike: range 11.68 (high 590.11 - low 578.43) more than 2% of low; 2026-06-26 15:59 EDT: a spike: range 16.825 (high 733.405 - low 716.58) more than 2% of low); the stores' clean series written from it: timescaledb-writer, clickhouse-writer; now 194092 K-lines, index 0 to 194091, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST; the damage after it 0
$ bin/query bars -stack test -topic market.clean.despike.v0-1-0.massive.SPY.M1 -from 2026-09-15T10:00 -to 2026-09-15T10:04
America/New_York        UTC                index   open      high      low      close    volume        n     vw
2026-09-15T10:00-04:00  2026-09-15T14:00Z  178912  759.5406  759.5506  759.21   759.225  52684.86967   1282  759.3544
2026-09-15T10:01-04:00  2026-09-15T14:01Z  178913  759.22    759.26    759.04   759.18   89638.96978   1450  759.145
2026-09-15T10:02-04:00  2026-09-15T14:02Z  178914  759.16    759.2     759.015  759.035  57566.243019  1098  759.1053
2026-09-15T10:03-04:00  2026-09-15T14:03Z  178915  759.04    759.12    758.9    758.95   95852.186291  1347  758.9889
2026-09-15T10:04-04:00  2026-09-15T14:04Z  178916  758.91    759.1     758.83   758.83   73615.613903  1246  758.9942
```

The four highs are the corrections'; 10:04's is Massive's.

## 4. The reset, with a reader running

As the reset stands today, a reader of the clean topic in a group holds it off. A reset deletes the topic with every
group's offsets on it, so that a reader started again reads the topic made again from its start, as if nothing had
happened; a reader still running would go on from where it was. So acquisition makes no reset while a group of the
topic has a member: its run fails, naming the group, and the next run tries again. Stopping the reader and starting it
again is yours, since no process manager of the platform's does it yet.

The medium level's consumer, on this topic, in the background (your shell prints the job's number and pid, not
shown), its lines on stderr in a file of their own, then a wait until it says it is at the topic's end, a few seconds:

```
$ bin/clean-follower -topic market.clean.despike.v0-1-0.massive.SPY.M1 -out $QI_TUTORIAL/despike.txt 2> $QI_TUTORIAL/follower.log & FOLLOWER=$!
$ until grep -qs "at the topic's end" $QI_TUTORIAL/follower.log; do sleep 1; done
$ cat $QI_TUTORIAL/follower.log
clean-follower: market.clean.despike.v0-1-0.massive.SPY.M1: from its start, no offset committed in group clean-follower; /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/despike.txt written anew
clean-follower: at the topic's end as it stood at the start, offset 194092: 194092 taken, 0 copies dropped; following
```

Four more corrections, past the tolerance, and a run:

```
$ bin/correct-high -at 2026-09-15T10:04-04:00 -minutes 4
massive:SPY:M1 at 2026-09-15T10:04-04:00: high 759.1, now 759.11, corrected [high]; read 2026-11-08T09:46:28.496Z, on market.corrections
massive:SPY:M1 at 2026-09-15T10:05-04:00: high 759.07, now 759.08, corrected [high]; read 2026-11-08T09:46:28.496Z, on market.corrections
massive:SPY:M1 at 2026-09-15T10:06-04:00: high 758.99, now 759, corrected [high]; read 2026-11-08T09:46:28.496Z, on market.corrections
massive:SPY:M1 at 2026-09-15T10:07-04:00: high 758.765, now 758.775, corrected [high]; read 2026-11-08T09:46:28.496Z, on market.corrections
$ bin/acquisition -dataset massive-spy-1m -catalog $QI_TUTORIAL/advanced.yaml -brokers localhost:29092 -corrected "$TS" -store "$TS" -store "$CH"
massive-reader: SPY:M1, 21 spans in [1791552600000, 1793998740000]: 18623 rows given by Massive in 1 pages; 8190 printed, 0 not closed, 10433 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 0 saved, 1 kept already
acquisition: massive:SPY:M1: the record: 194100 minutes on market.raw, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 0 copies (0 of another value); 8 corrections on market.corrections, 0 copies (0 of another value); asked 2026-10-09 09:30 EDT to 2026-11-06 15:59 EST, 8190 minutes of the grid in 21 spans; 8190 lines; published 0 first versions on market.raw (index none; 0 zero K-lines), 0 corrections on market.corrections; 8190 unchanged
timescaledb-writer: -corrected massive:SPY:M1: 194100 minutes of klines_corrected
time=2026-11-08T17:46:44.207+08:00 level=ERROR msg=fatal err="massive:SPY:M1: the clean: the reader in group clean-follower of market.clean.despike.v0-1-0.massive.SPY.M1 is running: the reset waits for it stopped"
$ bin/query groups -stack test | grep -E '^group|despike'
group               state   topic                                          partition  committed  end     behind
clean-follower      Stable  market.clean.despike.v0-1-0.massive.SPY.M1     0          194092     194092  0
```

The run read the record and the TimescaleDB store's corrected raw, found the damage past the tolerance (the next
run's line names its four minutes), and stopped before deleting anything. The reader stopped (`wait` until it has
ended), the next run makes the reset:

```
$ kill $FOLLOWER; wait $FOLLOWER
$ cat $QI_TUTORIAL/follower.log
clean-follower: market.clean.despike.v0-1-0.massive.SPY.M1: from its start, no offset committed in group clean-follower; /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/despike.txt written anew
clean-follower: at the topic's end as it stood at the start, offset 194092: 194092 taken, 0 copies dropped; following
clean-follower: stopped: 194092 taken, 0 copies dropped; 194092 K-lines in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/despike.txt; committed offset 194092
$ bin/acquisition -dataset massive-spy-1m -catalog $QI_TUTORIAL/advanced.yaml -brokers localhost:29092 -corrected "$TS" -store "$TS" -store "$CH"
massive-reader: SPY:M1, 21 spans in [1791552600000, 1793998740000]: 18623 rows given by Massive in 1 pages; 8190 printed, 0 not closed, 10433 outside the spans; the pages in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/pages/SPY/M1, 0 saved, 1 kept already
acquisition: massive:SPY:M1: the record: 194100 minutes on market.raw, index 0 to 194099, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST, 0 copies (0 of another value); 8 corrections on market.corrections, 0 copies (0 of another value); asked 2026-10-09 09:30 EDT to 2026-11-06 15:59 EST, 8190 minutes of the grid in 21 spans; 8190 lines; published 0 first versions on market.raw (index none; 0 zero K-lines), 0 corrections on market.corrections; 8190 unchanged
timescaledb-writer: -corrected massive:SPY:M1: 194100 minutes of klines_corrected
timescaledb-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
clickhouse-writer: -clean market.clean.despike.v0-1-0.massive.SPY.M1: 194092 K-lines read, 0 copies dropped (0 of another value); the clean series: 194092 rows written, the series emptied first
acquisition: massive:SPY:M1: the clean: market.clean.despike.v0-1-0.massive.SPY.M1, by despike 0.1.0: reset at 2026-11-08T09:46:58.343Z, 4 minutes damaged, more than the tolerance 3: 2026-09-15 10:04 EDT, 2026-09-15 10:05 EDT, 2026-09-15 10:06 EDT, 2026-09-15 10:07 EDT; the topics derived from it deleted: none; made again from timescaledb-writer's corrected raw, 194100 minutes: 194092 published, 8 refused (2025-04-07 13:59 EDT: a spike: range 11.0949 (high 509.36 - low 498.2651) more than 2% of low; 2025-04-09 13:19 EDT: a spike: range 16.38 (high 515.05 - low 498.67) more than 2% of low; 2025-04-09 13:20 EDT: a spike: range 14.99 (high 522.29 - low 507.3) more than 2% of low; 2025-04-16 14:31 EDT: a spike: range 12.3154 (high 537.8854 - low 525.57) more than 2% of low; 2025-04-23 12:22 EDT: a spike: range 10.91 (high 545.1 - low 534.19) more than 2% of low; 2025-04-23 12:25 EDT: a spike: range 10.825 (high 545.095 - low 534.27) more than 2% of low; 2025-05-27 13:13 EDT: a spike: range 11.68 (high 590.11 - low 578.43) more than 2% of low; 2026-06-26 15:59 EDT: a spike: range 16.825 (high 733.405 - low 716.58) more than 2% of low); the stores' clean series written from it: timescaledb-writer, clickhouse-writer; now 194092 K-lines, index 0 to 194091, 2024-11-08 09:30 EST to 2026-11-06 15:59 EST; the damage after it 0
```

Started again, its lines in a new file, the reader finds no offset of its group on the topic, deleted with the old
one, and reads the topic made again from its start, its file written anew:

```
$ bin/clean-follower -topic market.clean.despike.v0-1-0.massive.SPY.M1 -out $QI_TUTORIAL/despike.txt 2> $QI_TUTORIAL/follower-again.log & FOLLOWER=$!
$ until grep -qs "at the topic's end" $QI_TUTORIAL/follower-again.log; do sleep 1; done
$ cat $QI_TUTORIAL/follower-again.log
clean-follower: market.clean.despike.v0-1-0.massive.SPY.M1: from its start, no offset committed in group clean-follower; /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/despike.txt written anew
clean-follower: at the topic's end as it stood at the start, offset 194092: 194092 taken, 0 copies dropped; following
$ kill $FOLLOWER; wait $FOLLOWER
$ bin/clean-reader -topic market.clean.despike.v0-1-0.massive.SPY.M1 | md5sum
clean-reader: market.clean.despike.v0-1-0.massive.SPY.M1: 194092 messages, 194092 K-lines, 0 copies dropped
3c82e8888e7866a669e29fb032359060  -
$ md5sum < $QI_TUTORIAL/despike.txt
3c82e8888e7866a669e29fb032359060  -
$ grep -E ' 2026-09-15T14:0[0-8]Z ' $QI_TUTORIAL/despike.txt
178912 2026-09-15T14:00Z 759.5406 759.5506 759.21 759.225 52684.86967
178913 2026-09-15T14:01Z 759.22 759.26 759.04 759.18 89638.96978
178914 2026-09-15T14:02Z 759.16 759.2 759.015 759.035 57566.243019
178915 2026-09-15T14:03Z 759.04 759.12 758.9 758.95 95852.186291
178916 2026-09-15T14:04Z 758.91 759.11 758.83 758.83 73615.613903
178917 2026-09-15T14:05Z 758.8105 759.08 758.81 758.89 42519.298364
178918 2026-09-15T14:06Z 758.89 759 758.67 758.6777 63378.748853
178919 2026-09-15T14:07Z 758.675 758.775 758.54 758.73 83345.627959
178920 2026-09-15T14:08Z 758.72 758.75 758.52 758.535 1045391.219944
```

Its file is the topic made again, the eight corrected highs in it, 10:08's Massive's.

## 5. Done

```
$ make stack-test-down
python3 dev/scripts/run.py stop test
acquisition: not running
timescaledb-writer: stopped, pid 621378
clickhouse-writer: stopped, pid 621436
nix run --inputs-from . services#stack-test -- down -v
warning: Git tree '/home/zzhang/dev/qi/github/qi-v2-platform' is dirty
 Container qitest-timescaledb Stopping 
 Container qitest-console Stopping 
 Container qitest-clickhouse Stopping 
 Container qitest-console Stopped 
 Container qitest-console Removing 
 Container qitest-console Removed 
 Container qitest-redpanda Stopping 
 Container qitest-redpanda Stopped 
 Container qitest-redpanda Removing 
 Container qitest-redpanda Removed 
 Container qitest-timescaledb Stopped 
 Container qitest-timescaledb Removing 
 Container qitest-timescaledb Removed 
 Container qitest-clickhouse Stopped 
 Container qitest-clickhouse Removing 
 Container qitest-clickhouse Removed 
 Volume services-test_timescaledb_data Removing 
 Volume services-test_redpanda_data Removing 
 Volume services-test_clickhouse_data Removing 
 Network services-test_qitest-network Removing 
 Volume services-test_timescaledb_data Removed 
 Volume services-test_redpanda_data Removed 
 Volume services-test_clickhouse_data Removed 
 Network services-test_qitest-network Removed 
$ rm -r $QI_TUTORIAL
```
