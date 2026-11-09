"""The K-line data sets' targets (docs/designs/design-klines-dataset-2026-11-09.md): each a step of a data set's
making, checking or publishing, of one lock in datasets/.

    python3 dev/scripts/dataset.py make LOCK           # make dataset: built twice from the stack, the lock, the folder
    python3 dev/scripts/dataset.py check LOCK          # make dataset-check: the image read back from a registry of crane's own
    python3 dev/scripts/dataset.py push LOCK           # make dataset-push: the owner's push to GHCR, a login of its own
    python3 dev/scripts/dataset.py remote-check LOCK   # make dataset-remote-check: GHCR read with no login
    python3 dev/scripts/dataset.py locks               # make check-datasets: every lock well formed, a released one its tag's

A version's folder, its files, its image's layout and its lock as made, is ~/.local/share/qi/datasets/PRODUCT/VERSION/,
not in git. crane is QI_CRANE (the Makefile's, the flake's nixpkgs'); every crane command runs with a Docker
configuration of its own, so that no login of the machine's is used but the push's own. Each program runs in the
foreground, so Ctrl-C reaches it; the check's registry is stopped at the end, however it ends, and found gone. Standard
library only.
"""
import getpass
import hashlib
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

FIELDS = ["dataset", "version", "image", "tag", "digest", "made", "files"]
MADE = ["topic", "zone", "from", "to"]
ENTRY = ["path", "sha256", "size", "klines"]
OWNER = "zhifengzhang-sz"
PRODUCT = re.compile(r"^[a-z0-9]+([._-][a-z0-9]+)*$")
SEMVER = re.compile(r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$")
HEX = re.compile(r"^[0-9a-f]{64}$")
MONTH = re.compile(r"^[0-9]{4}-(0[1-9]|1[0-2])\.jsonl$")


def stop(msg):
    sys.exit(f"dataset: {msg}")


def load(path):
    try:
        with open(path) as f:
            return json.load(f)
    except (OSError, ValueError) as e:
        stop(f"{path}: {e}")


def one(args):
    if len(args) != 1 or not args[0].endswith(".lock"):
        stop("one lock is needed, datasets/PRODUCT.lock (LOCK=)")
    return args[0]


def tag(lock):
    return f"{lock['dataset']}-{lock['version']}"


def released(path, lock):
    """The lock's text at its version's tag, or None when the tag is not there."""
    t = tag(lock)
    if subprocess.run(["git", "rev-parse", "-q", "--verify", f"refs/tags/{t}"], capture_output=True).returncode:
        return None
    r = subprocess.run(["git", "show", f"{t}:{path}"], capture_output=True, text=True)
    if r.returncode:
        stop(f"the tag {t} holds no {path}: {r.stderr.strip()}")
    return r.stdout


def home(lock):
    return os.path.join(os.path.expanduser("~/.local/share/qi/datasets"), lock["dataset"], lock["version"])


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


def tree(root):
    """Every file under root and its sha256, by its path in root."""
    out = {}
    for d, _, names in os.walk(root):
        for n in names:
            p = os.path.join(d, n)
            out[os.path.relpath(p, root)] = sha256(p)
    return out


def files_match(folder, lock, what):
    """Every file of the lock in folder, its sha256 and size the lock's, and no other file."""
    got = tree(folder)
    want = {e["path"]: e for e in lock["files"]}
    faults = [f"{p}: not in the lock" for p in sorted(set(got) - set(want))]
    faults += [f"{p}: missing" for p in sorted(set(want) - set(got))]
    for p in sorted(set(got) & set(want)):
        size = os.path.getsize(os.path.join(folder, p))
        if got[p] != want[p]["sha256"] or size != want[p]["size"]:
            faults.append(f"{p}: sha256 {got[p]}, {size} bytes; the lock's {want[p]['sha256']}, {want[p]['size']}")
    if faults:
        stop(f"{what}: not the lock's files:\n  " + "\n  ".join(faults))
    return len(got)


def layout_match(oci, lock):
    """The layout's manifest the lock's digest, and every blob its name's sha256."""
    try:
        with open(os.path.join(oci, "index.json")) as f:
            ix = json.load(f)
    except (OSError, ValueError) as e:
        stop(f"{oci}: no image layout ({e}): make dataset")
    ms = ix.get("manifests", [])
    if len(ms) != 1 or ms[0].get("digest") != lock["digest"]:
        stop(f"{oci}: its manifest {[m.get('digest') for m in ms]}, the lock's {lock['digest']}")
    for p, s in tree(os.path.join(oci, "blobs", "sha256")).items():
        if p != s:
            stop(f"{oci}: the blob {p} holds sha256 {s}")


def crane():
    c = os.environ.get("QI_CRANE", "")
    if not (os.path.isfile(c) and os.access(c, os.X_OK)):
        stop(f"no crane at QI_CRANE ('{c}'): the Makefile gives it, nixpkgs#crane at flake.lock's pin")
    return c


def docker_config():
    """A Docker configuration of its own, holding no login: crane reads it in place of the machine's."""
    d = tempfile.mkdtemp(prefix="dataset-docker-")
    with open(os.path.join(d, "config.json"), "w") as f:
        f.write("{}\n")
    env = {k: v for k, v in os.environ.items() if k != "REGISTRY_AUTH_FILE"}
    env["DOCKER_CONFIG"] = d
    return d, env


def run(cmd, env, **kw):
    r = subprocess.run(cmd, env=env, **kw)
    if r.returncode:
        stop(f"{' '.join(cmd)}: exit {r.returncode}" + (f": {r.stderr.strip()}" if kw.get("capture_output") else ""))
    return r


def pushed(c, oci, ref, env, work):
    """The layout pushed to ref by crane, and the digest the registry gives back."""
    refs = os.path.join(work, "refs")
    run([c, "push", oci, ref, "--image-refs", refs], env)
    with open(refs) as f:
        got = f.read().strip()
    return got.rsplit("@", 1)[-1]


def read_back(c, ref, lock, env, work):
    """The image read as a taker reads it: crane export by its digest, the tar taken out by tar, every file the lock's."""
    out = os.path.join(work, "export")
    os.mkdir(out)
    exp = subprocess.Popen([c, "export", ref, "-"], env=env, stdout=subprocess.PIPE)
    tar = subprocess.run(["tar", "-x", "-C", out], stdin=exp.stdout)
    exp.stdout.close()
    if exp.wait() or tar.returncode:
        stop(f"crane export {ref} - | tar -x: exit {exp.returncode}, {tar.returncode}")
    n = files_match(out, lock, ref)
    run([c, "validate", "--remote", ref], env, capture_output=True, text=True)
    return n


def make(args):
    path = one(args)
    lock = load(path)
    for k in ["dataset", "version", "image", "made"]:
        if k not in lock:
            stop(f"{path}: no {k}")
    m = lock["made"]
    bin_ = os.path.abspath("bin/dataset")
    work = tempfile.mkdtemp(prefix="dataset-")
    try:
        runs = []
        for i in (1, 2):
            out = os.path.join(work, f"run{i}")
            cmd = [bin_, "-topic", m["topic"], "-zone", m["zone"], "-from", m["from"], "-to", m["to"],
                   "-name", lock["dataset"], "-version", lock["version"], "-image", lock["image"], "-out", out]
            if subprocess.run(cmd).returncode:
                stop(f"run {i}: {' '.join(cmd)}: failed")
            runs.append(out)
        a, b = tree(runs[0]), tree(runs[1])
        if a != b:
            diff = sorted(p for p in set(a) | set(b) if a.get(p) != b.get(p))
            stop(f"made twice, not the same bytes: {', '.join(diff)}")
        with open(os.path.join(runs[0], lock["dataset"] + ".lock")) as f:
            made_text = f.read()
        was = released(path, lock)
        with open(path) as f:
            now = f.read()
        if was is not None:
            if made_text != was or now != was:
                stop(f"{tag(lock)} is released and never changes: the lock made now, or {path}, is not the tag's; a "
                     "change is a new version")
            state = f"the tag {tag(lock)}'s, made again the same"
        elif made_text != now:
            with open(path, "w") as f:
                f.write(made_text)
            state = "written"
        else:
            state = "unchanged"
        dest = home(lock)
        if os.path.exists(dest):
            shutil.rmtree(dest)
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        shutil.copytree(runs[0], dest)
        print(f"dataset: {lock['dataset']} {lock['version']}: made twice, the same bytes ({len(a)} files); "
              f"{path} {state}; the version at {dest}")
    finally:
        shutil.rmtree(work, ignore_errors=True)


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def listening(port):
    with socket.socket() as s:
        return s.connect_ex(("127.0.0.1", port)) == 0


def check(args):
    path = one(args)
    lock = load(path)
    c = crane()
    dest = home(lock)
    layout_match(os.path.join(dest, "oci"), lock)
    files_match(os.path.join(dest, "files"), lock, os.path.join(dest, "files"))
    cfg, env = docker_config()
    work = tempfile.mkdtemp(prefix="dataset-check-")
    port = free_port()
    reg = subprocess.Popen([c, "registry", "serve", "--address", f"127.0.0.1:{port}"], env=env,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        for _ in range(100):
            if reg.poll() is not None:
                stop(f"crane registry serve: exit {reg.returncode}")
            if listening(port):
                break
            time.sleep(0.1)
        else:
            stop(f"crane registry serve: not listening on 127.0.0.1:{port} after 10 s")
        repo = f"127.0.0.1:{port}/{lock['dataset']}"
        got = pushed(c, os.path.join(dest, "oci"), f"{repo}:{lock['tag']}", env, work)
        if got != lock["digest"]:
            stop(f"{repo}:{lock['tag']}: pushed as {got}, the lock's {lock['digest']}")
        d = run([c, "digest", f"{repo}:{lock['tag']}"], env, capture_output=True, text=True).stdout.strip()
        if d != lock["digest"]:
            stop(f"{repo}:{lock['tag']}: its digest {d}, the lock's {lock['digest']}")
        n = read_back(c, f"{repo}@{lock['digest']}", lock, env, work)
    finally:
        if reg.poll() is None:
            reg.send_signal(signal.SIGTERM)
            try:
                reg.wait(10)
            except subprocess.TimeoutExpired:
                reg.kill()
                reg.wait()
        shutil.rmtree(work, ignore_errors=True)
        shutil.rmtree(cfg, ignore_errors=True)
    if reg.poll() is None or listening(port):
        stop(f"the registry on 127.0.0.1:{port} still running")
    print(f"dataset-check: {lock['dataset']}:{lock['tag']}: pushed to a registry of crane's own as {lock['digest']}, "
          f"the lock's; read back by its digest, every file ({n}) the lock's sha256 and size and no other; the "
          "registry stopped, nothing left")


def push(args):
    path = one(args)
    lock = load(path)
    c = crane()
    was = released(path, lock)
    with open(path) as f:
        now = f.read()
    if was is None or now != was:
        stop(f"{path}: not the lock of a released version as its tag {tag(lock)} holds it; only a release is pushed")
    dest = home(lock)
    layout_match(os.path.join(dest, "oci"), lock)
    files_match(os.path.join(dest, "files"), lock, os.path.join(dest, "files"))
    ref = f"{lock['image']}:{lock['tag']}"
    cfg, env = docker_config()
    work = tempfile.mkdtemp(prefix="dataset-push-")
    try:
        token = getpass.getpass(f"a token of {OWNER}'s that can write packages to ghcr.io (not shown, not kept): ")
        run([c, "auth", "login", "ghcr.io", "-u", OWNER, "--password-stdin"], env, input=token, text=True,
            capture_output=True)
        del token
        got = pushed(c, os.path.join(dest, "oci"), ref, env, work)
    finally:
        shutil.rmtree(cfg, ignore_errors=True)
        shutil.rmtree(work, ignore_errors=True)
    if got != lock["digest"]:
        stop(f"{ref}: GHCR gives back {got}, the lock's {lock['digest']}: not published as its lock says")
    print(f"dataset-push: {ref}: pushed, {got}, the lock's; the login removed. Next: the package set public on its "
          f"page, https://github.com/users/{OWNER}/packages/container/{lock['dataset']}/settings, then make "
          "dataset-remote-check")


def remote_check(args):
    path = one(args)
    lock = load(path)
    c = crane()
    was = released(path, lock)
    with open(path) as f:
        now = f.read()
    if was is None or now != was:
        stop(f"{path}: not the lock of a released version as its tag {tag(lock)} holds it; GHCR is held to that alone")
    cfg, env = docker_config()
    work = tempfile.mkdtemp(prefix="dataset-remote-")
    try:
        d = run([c, "digest", f"{lock['image']}:{lock['tag']}"], env, capture_output=True, text=True).stdout.strip()
        if d != lock["digest"]:
            stop(f"{lock['image']}:{lock['tag']}: its digest {d}, the lock's {lock['digest']}")
        n = read_back(c, f"{lock['image']}@{lock['digest']}", lock, env, work)
    finally:
        shutil.rmtree(work, ignore_errors=True)
        shutil.rmtree(cfg, ignore_errors=True)
    print(f"dataset-remote-check: {lock['image']}:{lock['tag']}: {lock['digest']}, the lock's, read with no login; "
          f"every file ({n}) the lock's sha256 and size and no other")


def faults_of(path, text):
    try:
        lock = json.loads(text)
    except ValueError as e:
        return [f"not JSON: {e}"]
    if not isinstance(lock, dict) or list(lock) != FIELDS:
        return [f"its fields {list(lock) if isinstance(lock, dict) else lock}, not {FIELDS}"]
    f = []
    if text != json.dumps(lock, indent=2, ensure_ascii=False) + "\n":
        f.append("not in its form: JSON indented by two spaces, a line feed at its end")
    if os.path.basename(path) != f"{lock['dataset']}.lock" or not PRODUCT.match(str(lock["dataset"])):
        f.append(f"dataset {lock['dataset']!r}: an image's name, the lock's file's")
    if not SEMVER.match(str(lock["version"])):
        f.append(f"version {lock['version']!r}: MAJOR.MINOR.PATCH")
    if lock["image"] != f"ghcr.io/{OWNER}/{lock['dataset']}":
        f.append(f"image {lock['image']!r}: ghcr.io/{OWNER}/{lock['dataset']}")
    if lock["tag"] != lock["version"]:
        f.append(f"tag {lock['tag']!r}: its version")
    if not (isinstance(lock["digest"], str) and lock["digest"].startswith("sha256:") and HEX.match(lock["digest"][7:])):
        f.append(f"digest {lock['digest']!r}: sha256: and 64 hex digits")
    if not isinstance(lock["made"], dict) or list(lock["made"]) != MADE:
        f.append(f"made: its fields {MADE}")
    es = lock["files"]
    if not isinstance(es, list) or not es:
        return f + ["files: none"]
    for e in es:
        if not isinstance(e, dict) or list(e) != ENTRY or not MONTH.match(str(e["path"])) or \
                not HEX.match(str(e["sha256"])) or not isinstance(e["size"], int) or e["size"] < 1 or \
                not isinstance(e["klines"], int) or e["klines"] < 1:
            f.append(f"files: {e}: {ENTRY}, its path YYYY-MM.jsonl, 64 hex digits, a size and K-lines above 0")
    paths = [e.get("path") for e in es if isinstance(e, dict)]
    if paths != sorted(set(paths), key=str):
        f.append("files: not in the order of their paths, each once")
    return f


def locks(args):
    if args:
        stop("locks takes no argument")
    paths = sorted(os.path.join("datasets", n) for n in os.listdir("datasets") if n.endswith(".lock")) \
        if os.path.isdir("datasets") else []
    bad = False
    for p in paths:
        with open(p) as f:
            text = f.read()
        faults = faults_of(p, text)
        if not faults:
            lock = json.loads(text)
            was = released(p, lock)
            if was is not None and was != text:
                faults.append(f"{tag(lock)} is released, and this is not its tag's lock")
            state = f"released, {tag(lock)}" if was is not None else "not released"
        for x in faults:
            print(f"check-datasets: {p}: {x}")
        if faults:
            bad = True
        else:
            print(f"check-datasets: {p}: well formed, {state}")
    if not paths:
        print("check-datasets: no lock in datasets/")
    sys.exit(1 if bad else 0)


def ended(signum, _frame):
    """SIGTERM or SIGHUP (the terminal closed) ends a step as Ctrl-C does, so that its cleanup runs: the push's login
    and the check's registry are removed however the step ends."""
    raise SystemExit(f"dataset: stopped by {signal.Signals(signum).name}")


def main():
    steps = {"make": make, "check": check, "push": push, "remote-check": remote_check, "locks": locks}
    if len(sys.argv) < 2 or sys.argv[1] not in steps:
        sys.exit(__doc__)
    for s in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(s, ended)
    steps[sys.argv[1]](sys.argv[2:])


if __name__ == "__main__":
    main()
