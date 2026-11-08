# Medium: a consumer on the core

A consumer that follows the clean topic, the basic level's (`../basic/README.md`), on the test stack it left up, with
`QI_TUTORIAL` set as it set it.

## 1. What it does

`clean-follower/main.go` is built on the core's `stream` package (`../../../core/README.md`; inside this repository,
since the core is not yet taken outside it) and its clean market data schema (`core/clean`). It follows a clean topic
and keeps each K-line it takes as a line of its file, the same line `clean-reader` prints. The rules it keeps are the
core's:

- **Its identity.** It reads in the group of its name, `clean-follower` (`-name`), as `core/README.md`'s rule on a
  component's identity says; its group's committed offset on the topic is where it takes up.
- **It commits after what it has done.** After each fetch its lines are written and the file synced, and only then is
  the last offset committed (`Source.Commit`). A stop between the two leaves lines whose offsets are not committed:
  they are delivered again, and dropped.
- **It drops a copy**, a value of a key and a date it has taken already, the first taken, as the schema's delivery says
  (delivery is at least once). A clean topic is one series', each value's key the topic's, so its key and date are its
  date, and its file, one topic's, holds the dates taken: one file a topic. Its file is what it has done: started with
  an offset committed in its group, it reads the dates of its file and continues from the offset; started with none,
  its first run or the topic made again (the advanced level), it reads the topic from its start and writes its file
  anew. A copy dropped is a line on stderr, `offset N: a copy of T, dropped`.
- **It follows the topic** until it is stopped (Ctrl-C, or SIGTERM), or until it has taken N K-lines in a run
  (`-max`), and says when it is at the topic's end as it stood when it started.

## 2. Stopped in the middle, and started again

```
$ go build -o bin/ ./docs/tutorial/medium/clean-follower
$ bin/clean-follower -topic market.clean.validation.v0-1-0.massive.SPY.M1 -out $QI_TUTORIAL/follower.txt -max 100000
clean-follower: market.clean.validation.v0-1-0.massive.SPY.M1: from its start, no offset committed in group clean-follower; /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/follower.txt written anew
clean-follower: stopped: 100000 taken, 0 copies dropped; 100000 K-lines in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/follower.txt; committed offset 100000
```

It has taken the first 100,000 K-lines and committed after the last; its group is behind by the rest:

```
$ bin/query groups -stack test | grep -E '^group|clean-follower'
group               state   topic                                          partition  committed  end     behind
clean-follower      Empty   market.clean.validation.v0-1-0.massive.SPY.M1  0          100000     194100  94100
```

Started again, it continues from its committed offset, takes the rest, and follows the topic; Ctrl-C stops it once it
says it is at the end:

```
$ bin/clean-follower -topic market.clean.validation.v0-1-0.massive.SPY.M1 -out $QI_TUTORIAL/follower.txt
clean-follower: market.clean.validation.v0-1-0.massive.SPY.M1: from offset 100000, group clean-follower's committed offset; 100000 K-lines in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/follower.txt
clean-follower: at the topic's end as it stood at the start, offset 194100: 94100 taken, 0 copies dropped; following
^C
clean-follower: stopped: 94100 taken, 0 copies dropped; 194100 K-lines in /tmp/nix-shell.fXHwWP/tmp.KhyxFBYjXp/follower.txt; committed offset 194100
```

No K-line taken twice and none missed: its file over both runs is the topic, as `clean-reader` read it whole.

```
$ wc -l < $QI_TUTORIAL/follower.txt
194100
$ md5sum < $QI_TUTORIAL/follower.txt
1d0b7a6b60450d2c774ee535d0bc04d9  -
$ md5sum < $QI_TUTORIAL/clean.txt
1d0b7a6b60450d2c774ee535d0bc04d9  -
```

The test stack stays up for the next level, `../advanced/README.md`. To stop here instead: `make stack-test-down` and
`rm -r $QI_TUTORIAL`.
