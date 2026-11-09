"""The project's checks for a change: the make targets that check the paths it touches, run one after another.

What a commit requires is the project's, not an agent's: the pre-commit hook (dev/hooks/pre-commit) runs this on
the staged paths, for a commit made by any kind of agent or by hand.

    python3 dev/scripts/verify.py            # the staged paths (the hook)
    python3 dev/scripts/verify.py --all      # every target of the table
    python3 dev/scripts/verify.py --since REF

It checks the work tree, not the index: a file staged in part is checked as it stands. A path no target checks
(the documents, the Makefile, this script) is named and passes: a commit of only such paths runs no
target. Standard library only.
"""
import subprocess
import sys

# a path's prefix and the targets that check it, in the order they run
TABLE = [
    # the core, the library and the additions (core/README.md, lib/README.md), and the stack's state their tests read
    ("core/", ["check-core"]),
    ("lib/", ["check-core"]),
    ("publishers/", ["check-core"]),
    ("consumers/", ["check-core"]),
    ("plugins/", ["check-core"]),
    # the charting consumer's page in a browser: its program's and its page's sources, and the flake, which builds the
    # page from visualization's packages (docs/designs/design-chart-consumer-2026-11-09.md, How it is checked)
    ("consumers/chart/", ["check-chart"]),
    ("flake.nix", ["check-chart"]),
    ("flake.lock", ["check-chart"]),
    # the tutorial's pages and its programs, a Go module each (docs/tutorial/README.md)
    ("docs/tutorial/", ["check-tutorial"]),
    ("dev/stack-state.mk", ["check-core"]),
    # the programs read config/ (acquisition and query the catalog)
    ("config/", ["check-core"]),
    # the data sets' locks, each held to its form and, released, to its tag's (dev/scripts/dataset.py locks)
    ("datasets/", ["check-datasets"]),
    ("dev/scripts/dataset.py", ["check-datasets"]),
    ("agents/", ["agents-check"]),
    # what PRODUCTS.md states against what the project has, read from these files
    ("PRODUCTS.md", ["products-check"]),
    ("flake.lock", ["products-check"]),
]


def paths(args):
    if args == ["--all"]:
        return None
    if len(args) == 2 and args[0] == "--since":
        cmd = ["git", "diff", "--name-only", "--no-renames", "-z", args[1]]
    elif not args:
        cmd = ["git", "diff", "--cached", "--name-only", "--no-renames", "-z"]
    else:
        sys.exit(__doc__)
    # -z: a path as it is; --no-renames: a moved file's old path is checked too
    return [p for p in subprocess.run(cmd, capture_output=True, text=True, check=True).stdout.split("\0") if p]


def targets(ps):
    if ps is None:
        return list(dict.fromkeys(t for _, ts in TABLE for t in ts)), []
    wanted = [t for pre, ts in TABLE if any(p.startswith(pre) for p in ps) for t in ts]
    unchecked = [p for p in ps if not any(p.startswith(pre) for pre, _ in TABLE)]
    return list(dict.fromkeys(wanted)), unchecked


def main():
    ps = paths(sys.argv[1:])
    ts, unchecked = targets(ps)
    for p in unchecked:
        print(f"verify: not checked by any target: {p}")
    results = [(t, subprocess.run(["make", t]).returncode) for t in ts]
    for t, rc in results:
        print(f"verify: make {t}: {'PASS' if rc == 0 else f'FAIL (exit {rc})'}")
    if not ts:
        print("verify: no target checks these paths")
    sys.exit(1 if any(rc for _, rc in results) else 0)


if __name__ == "__main__":
    main()
