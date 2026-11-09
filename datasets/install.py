"""datasets-install: a K-line data set's version installed into the Nix store, as a Nix taker takes it
(docs/designs/design-klines-dataset-2026-11-09.md, How a Nix taker takes a version).

    datasets-install DATASET VERSION

Run by the platform's flake package `datasets` (nix run <the platform>#datasets -- DATASET VERSION), which gives it
the locks of the commit it is built from and crane, both before DATASET:

    python3 -I install.py LOCKS CRANE DATASET VERSION

DATASET's lock, LOCKS/DATASET.lock, is VERSION, else it stops. Each file's store path is the one its name and sha256
fix; a file the store holds is not fetched again, and with every file there nothing is fetched and crane is not run.
Each file the store lacks is fetched from GHCR by crane with a Docker configuration of its own, empty, so that no login
is used: the image's manifest by the lock's digest, then the file's layer by its digest; it is added by the taker's own
nix-store only once it is the lock's sha256 and size. Ends 0 with every file in the store, 2 on its arguments, 1 on
any other fault, named. Standard library only.
"""
import hashlib
import io
import json
import os
import re
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile

NAME = "datasets-install"
USAGE = f"usage: {NAME} DATASET VERSION (nix run <the platform>#datasets -- DATASET VERSION)"
PRODUCT = re.compile(r"^[a-z0-9]+([._-][a-z0-9]+)*$")
SEMVER = re.compile(r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$")
HEX = re.compile(r"^[0-9a-f]{64}$")
MONTH = re.compile(r"^[0-9]{4}-(0[1-9]|1[0-2])\.jsonl$")
MANIFEST = "application/vnd.oci.image.manifest.v1+json"
LAYER = "application/vnd.oci.image.layer.v1.tar+gzip"


def stop(msg):
    sys.exit(f"{NAME}: {msg}")


def sha256(b):
    return hashlib.sha256(b).hexdigest()


def run(cmd, env=None):
    """cmd's output as bytes; a command that fails stops the install, named, with what it said."""
    r = subprocess.run(cmd, env=env, capture_output=True)
    if r.returncode:
        stop(f"{' '.join(cmd)}: exit {r.returncode}: {r.stderr.decode(errors='replace').strip()}")
    return r.stdout


def lock_of(locks, dataset, version):
    """DATASET's lock at the commit, its version VERSION, its digest and its files held to the lock's form."""
    if not PRODUCT.match(dataset):
        stop(f"{dataset!r}: not a data set's name (lower-case letters and digits, a '.', '_' or '-' between them)")
    path = os.path.join(locks, dataset + ".lock")
    if not os.path.isfile(path):
        have = sorted(n[: -len(".lock")] for n in os.listdir(locks) if n.endswith(".lock"))
        stop(f"no data set {dataset} in the platform at this commit; its data sets: {', '.join(have) or 'none'}")
    try:
        with open(path) as f:
            lock = json.load(f)
    except (OSError, ValueError) as e:
        stop(f"{path}: {e}")
    if lock.get("version") != version:
        stop(f"{dataset} at this commit is {lock.get('version')}, not {version}: a version is taken at a tag of the "
             "platform whose lock is that version")
    digest, files = lock.get("digest"), lock.get("files")
    if lock.get("dataset") != dataset or not isinstance(lock.get("image"), str) or not isinstance(digest, str) or \
            not digest.startswith("sha256:") or not HEX.match(digest[7:]) or not isinstance(files, list) or not files:
        stop(f"{path}: not a data set's lock (its dataset, image, digest and files)")
    for e in files:
        if not isinstance(e, dict) or not MONTH.match(str(e.get("path"))) or not HEX.match(str(e.get("sha256"))) or \
                not isinstance(e.get("size"), int) or e["size"] < 1:
            stop(f"{path}: files: {e}: a path YYYY-MM.jsonl, a sha256 of 64 hex digits and a size above 0")
    if len({e["path"] for e in files}) != len(files):
        stop(f"{path}: files: a path twice")
    return lock


def one_file(blob, path, what):
    """The bytes of the layer's one file, a regular file named path, else it stops."""
    try:
        with tarfile.open(fileobj=io.BytesIO(blob), mode="r:gz") as t:
            ms = t.getmembers()
            if len(ms) != 1 or ms[0].name != path or not ms[0].isreg():
                stop(f"{what}: not the one file {path}: {[(m.name, m.type) for m in ms]}")
            return t.extractfile(ms[0]).read()
    except (tarfile.TarError, OSError, EOFError) as e:
        stop(f"{what}: not a gzip tar: {e}")


def install(locks, crane, dataset, version):
    lock = lock_of(locks, dataset, version)
    files = lock["files"]
    nix_store = shutil.which("nix-store")
    if nix_store is None:
        stop("no nix-store on PATH: the taker's own Nix adds the files to its store")
    paths = [run([nix_store, "--print-fixed-path", "sha256", e["sha256"], e["path"]]).decode().strip() for e in files]
    invalid = set(run([nix_store, "--check-validity", "--print-invalid", *paths]).decode().split())
    want = [i for i, p in enumerate(paths) if p in invalid]
    fetched = set()
    if want:
        work = tempfile.mkdtemp(prefix=f"{NAME}-")
        try:
            cfg = os.path.join(work, "docker")
            os.mkdir(cfg)
            with open(os.path.join(cfg, "config.json"), "w") as f:
                f.write("{}\n")
            env = {k: v for k, v in os.environ.items() if k != "REGISTRY_AUTH_FILE"}
            env["DOCKER_CONFIG"] = cfg
            image = lock["image"]
            raw = run([crane, "manifest", f"{image}@{lock['digest']}"], env)
            if "sha256:" + sha256(raw) != lock["digest"]:
                stop(f"{image}@{lock['digest']}: the manifest given is sha256:{sha256(raw)}")
            try:
                m = json.loads(raw)
            except ValueError as e:
                stop(f"{image}@{lock['digest']}: its manifest not JSON: {e}")
            layers = m.get("layers") if isinstance(m, dict) else None
            if not isinstance(layers, list) or m.get("mediaType") != MANIFEST or len(layers) != len(files) or \
                    not all(isinstance(x, dict) and isinstance(x.get("digest"), str) for x in layers):
                stop(f"{image}@{lock['digest']}: not an image manifest of a layer a file ({len(files)})")
            for i in want:
                e, layer = files[i], layers[i]
                what = f"{image}@{layer['digest']}, the layer of {e['path']}"
                if layer.get("mediaType") != LAYER:
                    stop(f"{what}: its media type {layer.get('mediaType')}, not {LAYER}")
                blob = run([crane, "blob", f"{image}@{layer['digest']}"], env)
                if "sha256:" + sha256(blob) != layer["digest"] or len(blob) != layer.get("size"):
                    stop(f"{what}: given sha256:{sha256(blob)}, {len(blob)} bytes; the manifest's {layer.get('size')}")
                data = one_file(blob, e["path"], what)
                if sha256(data) != e["sha256"] or len(data) != e["size"]:
                    stop(f"{what}: {e['path']} is sha256 {sha256(data)}, {len(data)} bytes; the lock's {e['sha256']}, "
                         f"{e['size']}")
                d = os.path.join(work, str(i))
                os.mkdir(d)
                with open(os.path.join(d, e["path"]), "wb") as f:
                    f.write(data)
                got = run([nix_store, "--add-fixed", "sha256", os.path.join(d, e["path"])]).decode().strip()
                if got != paths[i]:
                    stop(f"{e['path']}: added as {got}, not {paths[i]}")
                fetched.add(i)
        finally:
            shutil.rmtree(work, ignore_errors=True)
    for i, e in enumerate(files):
        print(f"{NAME}: {e['path']}: {'fetched from GHCR' if i in fetched else 'in the store'}, {paths[i]}")
    n, k = len(files), len(fetched)
    what = f"fetched {k} from GHCR with no login, {n - k} in the store already" if k else "nothing fetched"
    print(f"{NAME}: {dataset} {version} ({lock['image']}@{lock['digest']}): every file ({n}) in the Nix store, the "
          f"lock's sha256; {what}")


def ended(signum, _frame):
    """SIGTERM or SIGHUP ends the install as Ctrl-C does, so that its temporary folders are removed."""
    raise SystemExit(f"{NAME}: stopped by {signal.Signals(signum).name}")


def main():
    for s in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(s, ended)
    args = sys.argv[1:]
    if len(args) != 4 or args[2] in ("-h", "--help"):
        print(USAGE, file=sys.stderr)
        sys.exit(2)
    locks, crane, dataset, version = args
    if not os.path.isdir(locks) or not (os.path.isfile(crane) and os.access(crane, os.X_OK)):
        stop(f"no locks' folder at {locks!r} or no crane at {crane!r}: run it by the platform's flake package datasets")
    if not SEMVER.match(version):
        print(f"{NAME}: {version!r}: not a version, MAJOR.MINOR.PATCH\n{USAGE}", file=sys.stderr)
        sys.exit(2)
    try:
        install(locks, crane, dataset, version)
    except KeyboardInterrupt:
        sys.exit(f"{NAME}: stopped by Ctrl-C")


if __name__ == "__main__":
    main()
