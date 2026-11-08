"""run.py start|stop STACK: the platform's components on a stack, each a process of its own in the background
(docs/impl/engineering/dev.environment.md, 3; the Makefile's start, stack-down, stack-test-stop and stack-test-down).

The components: acquisition, following its dataset (-follow: a run at its start, then one whenever the next session's
last bar is the dataset's delay old), its reset reading the TimescaleDB store's corrected raw and writing each store's
clean series by the writers' programs (-corrected, -store); and the two stores' writers, each following the core's
topics from its group's committed offsets. Each one's log and pid are in the stack's run folder, ~/.local/state/qi/run/STACK/; each start
appends a line to each log naming the start, then the component's own lines. Where each takes up: acquisition's run line
(the record, the span asked); each writer's group's committed offsets, read before it starts (rpk in the stack's
Redpanda, no group joined), and its log, which says whether it reads from them or from the topics' start.

start provisions the core's topics (bin/provision) and starts each component not running, in a session of its own, so
that it keeps running after make ends; stop stops each running (SIGTERM), waiting for it to end, so that the stack goes
down with no component writing. A component that has ended on its own (a writer's fault, the stack gone) is not
running; its log says why, and the next start starts it.

STACK is stack (qicore-*, the platform's data) or test (qitest-*, every port the stack's plus 10000). CATALOG is the
catalog acquisition reads: config/catalog.yaml for the stack by default; for the test stack, given, a catalog of the run's
own whose plugin keeps its pages in a temporary folder (every test and trial run on the test stack keeps its state and
pages so). DATASET is acquisition's dataset, massive-spy-1m by default.
"""

import datetime
import os
import pathlib
import signal
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]
BIN = ROOT / "bin"

STACKS = {
    "stack": {"brokers": "localhost:19092", "redpanda": "qicore-redpanda",
              "timescaledb": "postgres://postgres:password@localhost:5432/postgres",
              "clickhouse": "http://default:password@localhost:8123"},
    "test": {"brokers": "localhost:29092", "redpanda": "qitest-redpanda",
             "timescaledb": "postgres://postgres:password@localhost:15432/postgres",
             "clickhouse": "http://default:password@localhost:18123"},
}


def fail(msg):
    print(f"run.py: {msg}", file=sys.stderr)
    sys.exit(1)


def components(stack, catalog, dataset):
    s = STACKS[stack]
    ts = f"{BIN / 'timescaledb-writer'} -brokers {s['brokers']} -url {s['timescaledb']}"
    ch = f"{BIN / 'clickhouse-writer'} -brokers {s['brokers']} -url {s['clickhouse']}"
    return [
        ("acquisition", [str(BIN / "acquisition"), "-dataset", dataset, "-catalog", catalog, "-brokers", s["brokers"],
                         "-corrected", ts, "-store", ts, "-store", ch, "-follow"]),
        ("timescaledb-writer", [str(BIN / "timescaledb-writer"), "-brokers", s["brokers"], "-url", s["timescaledb"]]),
        ("clickhouse-writer", [str(BIN / "clickhouse-writer"), "-brokers", s["brokers"], "-url", s["clickhouse"]]),
    ]


def committed(stack, group):
    """Where a writer takes up: its group's committed offset of each partition of the core's topics that holds records,
    and the partition's end, read by rpk in the stack's Redpanda (no group joined); None when the group has none (rpk
    describes a group unknown to it as Dead, no partition). rpk's fault stops the start."""
    out = subprocess.run(["docker", "exec", STACKS[stack]["redpanda"], "rpk", "group", "describe", group],
                         capture_output=True, text=True)
    if out.returncode != 0:
        fail(f"rpk group describe {group} in {STACKS[stack]['redpanda']}: exit {out.returncode}: "
             f"{(out.stderr or out.stdout).strip()}")
    rows = []
    for line in out.stdout.splitlines():
        f = line.split()
        # TOPIC PARTITION CURRENT-OFFSET LOG-START-OFFSET LOG-END-OFFSET LAG …
        if len(f) >= 6 and f[0] in ("market.raw", "market.corrections") and f[4] != "0":
            rows.append(f"{f[0]}/{f[1]} at {f[2]} of {f[4]}" if f[2] != "-" else f"{f[0]}/{f[1]} none of {f[4]}")
    return rows or None


def running(pidfile, program):
    """The pid in pidfile when that process is running program, else None."""
    try:
        pid = int(pidfile.read_text().strip())
        cmdline = pathlib.Path(f"/proc/{pid}/cmdline").read_bytes().split(b"\0")
    except (OSError, ValueError):
        return None
    return pid if cmdline and cmdline[0].decode() == program else None


def start(stack, run, catalog, dataset):
    for p in [BIN / "provision"] + [pathlib.Path(argv[0]) for _, argv in components(stack, catalog, dataset)]:
        if not p.is_file():
            fail(f"no {p}: make build")
    if not pathlib.Path(catalog).is_file():
        fail(f"no catalog {catalog}")
    subprocess.run([str(BIN / "provision"), "-brokers", STACKS[stack]["brokers"]], check=True)
    run.mkdir(parents=True, exist_ok=True)
    now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"
    for name, argv in components(stack, catalog, dataset):
        pidfile, log = run / f"{name}.pid", run / f"{name}.log"
        pid = running(pidfile, argv[0])
        if pid:
            print(f"{name}: running, pid {pid}, log {log}")
            continue
        if name != "acquisition":  # acquisition says where it takes up itself, in its run's line
            at = committed(stack, name)
            print(f"{name}: its group's committed offsets: {', '.join(at) if at else 'none'}; it takes up there, or at "
                  f"the topics' start when its store holds no bar (its log's from_start)")
        with open(log, "a") as out:
            out.write(f"== start {now} on the {stack}: {' '.join(argv)}\n")
            out.flush()
            p = subprocess.Popen(argv, cwd=ROOT, stdin=subprocess.DEVNULL, stdout=out, stderr=subprocess.STDOUT,
                                 start_new_session=True)
        pidfile.write_text(f"{p.pid}\n")
        print(f"{name}: started, pid {p.pid}, log {log}")
    time.sleep(2)
    for name, argv in components(stack, catalog, dataset):
        if not running(run / f"{name}.pid", argv[0]):
            fail(f"{name} ended at its start: its log, {run / (name + '.log')}")


def stop(stack, run, catalog, dataset):
    for name, argv in components(stack, catalog, dataset):
        pidfile = run / f"{name}.pid"
        pid = running(pidfile, argv[0])
        if not pid:
            print(f"{name}: not running")
            pidfile.unlink(missing_ok=True)
            continue
        os.kill(pid, signal.SIGTERM)
        for _ in range(1200):
            if not running(pidfile, argv[0]):
                break
            time.sleep(0.1)
        else:
            fail(f"{name}, pid {pid}: still running 120 s after SIGTERM")
        pidfile.unlink(missing_ok=True)
        print(f"{name}: stopped, pid {pid}")


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("start", "stop") or sys.argv[2] not in STACKS:
        fail("run.py start|stop stack|test")
    what, stack = sys.argv[1], sys.argv[2]
    run = pathlib.Path.home() / ".local" / "state" / "qi" / "run" / stack
    catalog = os.environ.get("CATALOG", "")
    if not catalog:
        if stack == "test" and what == "start":
            fail("the test stack's start needs CATALOG, a catalog of the run's own whose plugin keeps its pages in a "
                 "temporary folder (-pages)")
        catalog = str(ROOT / "config" / "catalog.yaml")
    dataset = os.environ.get("DATASET", "") or "massive-spy-1m"
    (start if what == "start" else stop)(stack, run, os.path.abspath(catalog), dataset)


if __name__ == "__main__":
    main()
