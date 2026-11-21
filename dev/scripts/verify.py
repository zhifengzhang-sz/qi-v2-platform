"""The project's checks for a change: the make targets that check the paths it touches, run one after another.

What a commit requires is the project's, not an agent's: make verify runs this after the kit's part, and the
pre-commit hook that make install writes runs make verify, for a commit made by any kind of agent or by hand.
PRODUCTS.md and a *.lock at the root are the kit's part (make products-check, run by make verify where one is
staged): this script counts them checked, and runs products-check itself only with --all or --since.

    python3 dev/scripts/verify.py            # the staged paths (make verify, the hook)
    python3 dev/scripts/verify.py --all      # every target of the table, and products-check
    python3 dev/scripts/verify.py --since REF

It checks the work tree, not the index: a file staged in part is checked as it stands. A path no target checks
(the documents, the Makefile, this script) is named and passes: a commit of only such paths runs no
target. Standard library only.
"""
import re
import subprocess
import sys

# the paths the kit's part of make verify checks, by make products-check, before this script runs
KIT = re.compile(r"PRODUCTS\.md|[^/]+\.lock")

# a path's prefix and the targets that check it, in the order they run
TABLE = [
    # the library (lib/README.md), and the flake, which gives it market's Go form and vectors
    ("lib/", ["check-lib"]),
    ("flake.nix", ["check-lib"]),
    ("flake.lock", ["check-lib"]),
    # the definition's interfaces (abstract/README.md), and the flake, which gives them the definition
    ("abstract/", ["check-abstract"]),
    ("flake.nix", ["check-abstract"]),
    ("flake.lock", ["check-abstract"]),
    # the package abstract-level (abstract/README.md, Taking it): what it holds, the flake that builds it from
    # nixpkgs, and the taker's test that takes it
    ("abstract/", ["check-abstract-package"]),
    ("LICENSE", ["check-abstract-package"]),
    ("flake.nix", ["check-abstract-package"]),
    ("flake.lock", ["check-abstract-package"]),
    ("dev/taker/abstract-level/", ["check-abstract-package"]),
    ("agents/", ["agents-check"]),
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


def targets(ps, staged):
    if ps is None:
        return list(dict.fromkeys([t for _, ts in TABLE for t in ts] + ["products-check"])), [], []
    wanted = [t for pre, ts in TABLE if any(p.startswith(pre) for p in ps) for t in ts]
    kit = [p for p in ps if KIT.fullmatch(p)]
    # staged, they are the kit's part of make verify, which has run products-check; else it is run here
    if kit and not staged:
        wanted.append("products-check")
    unchecked = [p for p in ps if p not in kit and not any(p.startswith(pre) for pre, _ in TABLE)]
    return list(dict.fromkeys(wanted)), unchecked, kit


def main():
    ps = paths(sys.argv[1:])
    ts, unchecked, kit = targets(ps, not sys.argv[1:])
    for p in unchecked:
        print(f"verify: not checked by any target: {p}")
    results = [(t, subprocess.run(["make", t]).returncode) for t in ts]
    for t, rc in results:
        print(f"verify: make {t}: {'PASS' if rc == 0 else f'FAIL (exit {rc})'}")
    if not ts and not kit:
        print("verify: no target checks these paths")
    sys.exit(1 if any(rc for _, rc in results) else 0)


if __name__ == "__main__":
    main()
