# The K-line data sets: a fixed set of a clean series' K-lines, a version each, an image on GHCR pinned by a lock, taken by Nix by their hashes (2026-11-09)

The platform publishes K-line data sets. A data set is a fixed set of K-lines of one of the platform's clean series,
over a range of its sessions, made from the record; each is a product with a version, and a version once released never
changes, so that anything made from a data set names the version it was made from and is made again from the same
bytes. It is published as `qi-v2-nix`'s `practices.md`, 4, has it: an image on GHCR, one layer a file, and a lock in git
naming the image's tag and every file's sha256; no data in git. The image is public, read with no login. The first data
set is SPY's minutes, `qi-v2-platform-klines-massive-spy-1m` 1.0.0. A Nix taker takes a version by the platform's
flake, its product `datasets`: its package installs the version's files into the Nix store from GHCR, and its function
`lib.dataset` gives the taker's own package each file by its hash. It rests on: the clean market data schema 0.1.0 and
the core's stream (`core/README.md`, `core/clean`, `core/stream`); market 1.1.0's K-line and its canonical text, as
the core reads it (`core/raw`) and the charting consumer writes it (`consumers/chart`); `practices.md`, 3, 4 and 7, of
`qi-v2-nix` at the commit `flake.lock` pins; crane, go-containerregistry's, 0.21.6, the flake's nixpkgs'; the OCI image
specification's layout, manifest and config; nixpkgs' `requireFile` and `linkFarm`, and Python 3.13's standard library,
the flake's nixpkgs'; the taker's own Nix, its `nix-store` (`--print-fixed-path`, `--check-validity`, `--add-fixed`).

## What this design guarantees

1. **A version is fixed.** A data set's version, once released (its tag `<product>-<version>` on the commit of its
   lock), never changes: its lock at the tag names the image's digest and every file's sha256 and size; `make
   dataset` refuses to write a lock of a released version that differs from the tag's, and `check-datasets`, in `make
   check`, holds every lock of a released version to its tag's. A taker names the image by its digest, never by its
   tag alone.
2. **Made from the record, read as the contract reads it.** A data set is made from one clean topic, read in no group
   from its start to its end as it stood when the read began: no topic written, no group joined, no offset committed;
   the stack, which holds the platform's data, read only.
3. **The same bytes again.** Made twice from the same topic, a data set is the same files, the same image layout and
   the same lock, byte for byte: nothing of time, of the machine or of the run is in any of them.
4. **Every K-line checked.** Each value is read as clean market data 0.1.0 (`clean.FromRecord`, every rule of the
   schema and of market's K-line); a copy is dropped, the first taken; each K-line's index the last one's and 1 and its
   date after the last one's, else the run stops, named; the topic covers the range, else the run stops.
5. **No data in git; public, with no login.** The files are in the image alone; git holds the lock. The image is public
   on GHCR, read with no login, as every part of its making is (the platform's own programs, the flake's nixpkgs).
6. **The push is the owner's.** The session builds and checks; the image reaches GHCR only by the owner's push, with a
   login of its own that is kept in a temporary folder for the push and removed after it.
7. **Ctrl-C reaches the program; nothing of a step is left.** Each program is run as its own process in the
   foreground; the check's registry is stopped at its end or at Ctrl-C, and found gone after; the install's temporary
   folders are removed however it ends.
8. **A Nix taker takes a version by its hashes, with no login.** A taker pins the platform's flake at a tag and names
   a data set's version, which the lock at that commit must be, or its evaluation stops, named. The install fetches
   from GHCR with a Docker configuration of its own, holding no login, and adds a file to the Nix store only once it is
   the lock's sha256 and size; `lib.dataset` gives the taker's package each file as the store path its name and sha256
   fix, which no other bytes can fill, so a package is built from the version's bytes or not at all. A file the store
   holds is not fetched again, and with every file there nothing is fetched.

## What a data set holds

**The series: the clean.** A data set holds one clean topic's K-lines: for SPY's minutes,
`market.clean.validation.v0-1-0.massive.SPY.M1`, the validation 0.1.0's clean series of `massive:SPY:M1`. Not the raw
(`market.raw`'s first versions) and not the corrected raw (each minute's latest version, a series of the stores, not a
topic): the clean topic is made from the corrected raw, so each minute is the latest version Massive has given; every
K-line in it has passed the validation (every price finite and above 0, `low ≤ min(open, close)`, `max(open, close) ≤
high`, `volume ≥ 0`, its date after the last accepted); and it leaves out the zero K-line, the place of a minute the
source gave nothing for, which on `market.raw` is a bar of prices 0. It is a topic of the contract (`README.md`, The
platform contract), one partition, its K-lines in the order of their dates.

**The range: whole sessions, by their dates in the series' zone.** A data set's range is its first and last session's
dates in its zone (`America/New_York` for SPY's), both included. SPY's 1.0.0 is 2024-11-06 to 2026-11-06, the clean
topic whole as the record holds it: the two years Massive's free plan gives, every session whole, from 2024-11-06 09:30
to 2026-11-06 15:59 New York time. Whether a range's ends are whole sessions is not known from the topic alone (the
catalog's calendar says which minutes a session has, and the clean series leaves out the zero K-line), so the release
holds it: the run gives its first and last K-line's time, and the release's review checks them against the session.

**The K-lines.** Each is market 1.1.0's K-line, its seven fields (`close`, `date`, `high`, `index`, `low`, `open`,
`volume`), the clean K-line's base, its `index` its place in the data set from 0. A source's own fields (Massive's `vw`
and `n`) are left out: a K-line with them is the source's, not market's, which `@qi/market`'s `klineSchema` refuses,
being strict; they stay on the topics, the record. SPY's 1.0.0 starts at the clean topic's first K-line, so each index
is its clean series' index too.

## Its form

**The files.** One file a calendar month of its sessions' dates in the zone, `YYYY-MM.jsonl` (`2024-11.jsonl`), each
holding that month's K-lines of the range in the order of their dates, one a line: market's canonical text of the
K-line, one JSON object of its seven fields in the order of their names, no spaces, each number as market writes it, a
line feed after each, UTF-8; the first and last months as much of them as the range holds. SPY's 1.0.0 has 25 files,
2024-11 to 2026-11. Why:

- **Market's own text.** A taker in any form of market reads a line by market's own rules: in TypeScript `JSON.parse`
  and `@qi/market`'s `klineSchema`, as the charting consumer's page reads each K-line it is fed, the same text; in Go
  the core's `raw.DecodeKLine`. Nothing of a format of the platform's own needs a reader written for it.
- **Text.** Any language reads it with no library; the image's layers are compressed (gzip), so the text costs little
  to fetch.
- **A month a file.** `practices.md`, 4: "data that changes often (market data) has assets of its own", so that a new
  version costs only the assets that changed. A later version that extends the range changes its last month and adds
  the months after; every month whose K-lines the record has not corrected is the same file, the same sha256, which a
  taker that holds it does not fetch again. A day a file would be about 500 layers of some 40 kB each, every one a
  request of its own to fetch; a year a file would make each new version change a third of the data.

**The image.** `ghcr.io/zhifengzhang-sz/<product>:<version>`, `ghcr.io/zhifengzhang-sz/qi-v2-platform-klines-massive-spy-1m:1.0.0`:
an OCI image manifest (`application/vnd.oci.image.manifest.v1+json`), one layer a file in the order of their names,
each layer a tar of its one file at the image's root (mode 0444, owner 0:0, its time 0), gzip-compressed with no time
and no name in its header; its config an OCI image config for `linux`/`amd64`, with no command, no entrypoint and no
`created`, its labels `org.opencontainers.image.source` (this repository, which links the package to it on GHCR),
`.title` (the product), `.version` and `.description`; the same four as the manifest's annotations. Data only: it runs
nothing. It is written as an OCI image layout by the program that makes the data set, so its digest, the manifest's, is
known before the push and is the same each time it is made; crane pushes a layout's manifest and blobs as they are, so
GHCR serves that digest. The files' sha256 depend on the topic alone; the layers' digests depend on Go's gzip too, the
flake's Go, so the image's digest is a build's of this flake.

**The lock.** `datasets/<product>.lock`, in git, one JSON object, indented, its fields in this order: `dataset` (the
product), `version`, `image` (the repository on GHCR), `tag` (the image's tag, the version), `digest` (the image's
manifest), `made` (what it is made from: `topic`, `zone`, `from`, `to`), `files` (each `path` in the image, `sha256`,
`size` in bytes, `klines`). `dataset`, `version`, `image` and `made` are what a version is made of, set by the session
for a new version; the rest is written by its making. "Lock" means pinned, not frozen (`practices.md`, 4): a new
version rewrites it, and committing it is the release. It is in `datasets/`, not at the root: a `*.lock` at the root is
one the project takes, which `PRODUCTS.md`'s Takes lists (`practices.md`, 7), and this one is of what it gives.

## Its versions

`MAJOR.MINOR.PATCH`, one number for the product, written alike in its lock, its tag, `PRODUCTS.md` and `datasets/README.md`
(Changes):

- **MAJOR**, its form or its series: the files, the line, the image's layout, or the clean topic it is made from (another
  cleaning, or another version of its cleaning), so that a reader of the version before reads it otherwise, or reads
  other K-lines;
- **MINOR**, its range: more sessions, and with them any K-line the record has corrected since;
- **PATCH**, its K-lines: the same form and range, a K-line the record has corrected since.

The first is 1.0.0. A version's tag is `<product>-<version>` (`qi-v2-platform-klines-massive-spy-1m-1.0.0`), on the
commit of its lock.

## How a taker pins one and reads it

- **Pins.** The lock at the version's tag: `datasets/<product>.lock` of `zhifengzhang-sz/qi-v2-platform` at
  `<product>-<version>`, read over HTTPS with no login
  (`https://raw.githubusercontent.com/zhifengzhang-sz/qi-v2-platform/<tag>/datasets/<product>.lock`), or, for a Nix
  taker, the platform's flake at a tag whose lock is that version, pinned by the taker's own lock (How a Nix taker
  takes a version, below). The image by the lock's digest, `<image>@<digest>`.
- **Reads**, with no login, crane (or any client of a registry) taking the files out:

      crane export ghcr.io/zhifengzhang-sz/qi-v2-platform-klines-massive-spy-1m@<digest> - | tar -x -C <folder>
      jq -r '.files[] | "\(.sha256)  \(.path)"' <product>.lock | (cd <folder> && sha256sum --check --strict)

  A Nix taker takes it by the platform's flake instead (below).
- **Names it.** Anything made from a data set names its product and version, and the digest, which the lock at the tag
  gives.

## How a Nix taker takes a version

The platform's flake gives a Nix taker each data set of the commit it pins by the product `datasets` (`practices.md`,
3 and 4): its package `datasets`, whose program `datasets-install` installs a version's files into the taker's Nix
store from GHCR, and its function `lib.dataset`, which gives the taker's own package each file by its hash. Both read
that commit's locks, `datasets/*.lock`, and nothing else of it. The function is of the product though no package: a
taker's package takes the files in the taker's own nixpkgs, which a package of the platform's could not.

**The taker's lines.** Its flake pins the platform at a tag and takes the version in its package:

    {
      inputs.platform.url = "github:zhifengzhang-sz/qi-v2-platform?ref=datasets-1.0.0";
      inputs.nixpkgs.follows = "platform/nixpkgs";
      outputs = { self, platform, nixpkgs }:
        let
          pkgs = nixpkgs.legacyPackages.x86_64-linux;
          spy = platform.lib.dataset {
            inherit pkgs;
            dataset = "qi-v2-platform-klines-massive-spy-1m";
            version = "1.0.0";
          };
        in {
          packages.x86_64-linux.default = pkgs.runCommand "spy-minutes" { } ''
            cat ${spy.folder}/*.jsonl | wc -l > $out
          '';
        };
    }

Then, in its folder, its lock written, the version installed from GHCR with no login by the platform's flake at the
commit that lock pins, and its package built:

    nix flake lock
    nix run --inputs-from . platform#datasets -- qi-v2-platform-klines-massive-spy-1m 1.0.0
    nix build

Its nixpkgs is its own or the platform's (`follows`, as here): a file's store path depends on its name and sha256
alone, never on the nixpkgs that takes it. The tag is one whose lock is the version it names, since a commit holds one
version of each data set, its lock's: a data set's own tag, `<product>-<version>`, for each version released after
`datasets` 1.0.0, the function at its commit; for SPY's 1.0.0, whose tag is older than the function, `datasets-1.0.0`.

**`lib.dataset { pkgs, dataset, version }`.** It reads `datasets/<dataset>.lock` at the commit, `dataset` a product's
name (lower-case letters and digits, a `.`, `_` or `-` between them), and stops the evaluation, named, when there is
no such lock (naming those there) or its version is not `version`. It gives `dataset`, `version`, `image` and
`digest`, the lock's, which anything made from the data set names; `files`, each file of the lock by its path,
`pkgs.requireFile { name = <path>; sha256 = <sha256>; }`, a fixed-output path, the one `nix-store --add-fixed sha256`
gives the file named by its path, which only those bytes fill; and `folder`, `pkgs.linkFarm "<dataset>-<version>"`,
one folder linking every file by its path. It fetches nothing: a file reaches the store by the install alone, and a
file not there stops the taker's build at that file, its message the line that installs the version.

**`datasets-install DATASET VERSION`**, run as `nix run <the platform>#datasets -- DATASET VERSION`:

1. The lock of `DATASET` at the commit, its version `VERSION`: a name with no lock or another version stops it, named,
   as it stops `lib.dataset`; its digest and its files' paths, sha256 and sizes held to the lock's form.
2. Each file's store path (`nix-store --print-fixed-path sha256 <sha256> <path>`), and those the store holds
   (`nix-store --check-validity --print-invalid`). With every one there it ends, saying so: nothing is fetched and
   crane is not run.
3. Else, with a Docker configuration of its own, empty, so that no login is used: the image's manifest by the lock's
   digest (`crane manifest <image>@<digest>`), its sha256 the digest, a layer for each file in the lock's order; for
   each file the store lacks, its layer by its digest (`crane blob <image>@<layer>`), its sha256 and size the
   manifest's, its tar one regular file named the file's path, the file's sha256 and size the lock's; then the file
   added (`nix-store --add-fixed sha256`), the store path given back the one computed.
4. A line each file, fetched from GHCR or in the store; a last line, the version, its image and digest, the files
   fetched and those there. It ends 0 with every file in the store; 2 on its arguments; 1 on any other fault, named, a
   file added only once it is checked. Ctrl-C, SIGTERM or SIGHUP stops it, 1, its temporary folders removed.

It is `datasets/install.py`, Python's standard library, run by the package with the flake's python3 (`-I`), crane the
flake's nixpkgs', and the taker's own `nix-store`, the first on its `PATH`, else it stops naming it. Why this form:
`practices.md`, 4, crane fetching outside Nix's daemon and the lock telling Nix what to expect, here with no login
since the data sets are public (What this design guarantees, 5); the store path its name and sha256 fix, so that a month a later version leaves unchanged is the same path and is not fetched again, and a new
version costs only the months that changed. The files are held by what takes them: a garbage collection of the store
removes a file nothing holds, and the same line installs it again.

**Its versions.** `datasets`' own number, `MAJOR.MINOR.PATCH`, written alike in this design, `flake.nix` (the
package's `version`), `PRODUCTS.md` and `datasets/README.md` (Changes): MAJOR, a taker's lines read otherwise (what
`lib.dataset` takes or gives, the install's arguments); MINOR, something added that a taker's lines need not name;
PATCH, a fault mended. The first is 1.0.0, its tag `datasets-1.0.0`. A data set's version does not change `datasets`'
number, nor `datasets`' a data set's.

## How it is made

**The program `dataset`** (`consumers/dataset`), a consumer on the core, a module in `CORE` on the core's stream and
clean schema, market's Go form and the standard library:

    dataset -topic TOPIC -zone ZONE -from DATE -to DATE -name PRODUCT -version VERSION -image REPOSITORY -out DIR
            [-brokers localhost:19092]

It reads the clean topic's span as it stands (one partition, a clean topic's), then the topic from its start in no
group (`stream.SubscribeOptions{Group: ""}`) to that end, each value checked (What this design guarantees, 4); it keeps
the K-lines whose session's date in the zone is in the range, the topic covering it (its first K-line's date on or
before `-from`, its last on or after `-to`), at least one; then it writes, into `DIR`, which it makes and which must not
exist: `files/`, the month files; `oci/`, the image's layout (`oci-layout`, `index.json` naming the manifest with its tag,
`blobs/sha256/`); `<product>.lock`. It ends 0 with a line: the K-lines read, the copies dropped, the data set's first
and last K-line in the zone, its files, K-lines and bytes, the image's digest; 2 on a flag missing or not known, 1 on
any other fault, a flag's value refused among them, named. Ctrl-C or SIGTERM stops it, 1. It names no source and no
series: any clean topic is made a data set by its flags. A K-line's text is the core's, `raw.FromBase`, the K-line of a
base alone, market's canonical text of its seven fields.

**The targets** (`Makefile`; their steps in `dev/scripts/dataset.py`, standard library only), each of the one lock in
`datasets/`, or `LOCK=` one of them; crane the flake's nixpkgs' (`nix build --inputs-from . nixpkgs#crane`), not in
the shell:

- `make dataset`: `bin/dataset` built; the lock's `made` read from the stack (`localhost:19092`) twice, each run into a
  temporary folder of its own; the two the same bytes, every file, or it stops; then, for a version not released, the
  lock written into `datasets/`, and for one released, the made lock the same as the tag's, or it stops; the version's
  folder, its files, its layout and its lock, put at `~/.local/share/qi/datasets/<product>/<version>/`, not in git.
- `make dataset-check`: the image read back. A registry of crane's own (`crane registry serve`, in memory) on
  127.0.0.1; the layout pushed to it (`crane push`), the digest it gives back the lock's; the image read as a taker
  reads it, by its digest (`crane export`, then the files taken out of the tar), every file of the lock there with its
  sha256 and size and no other; `crane validate --remote` of it; the registry stopped and found gone. Every crane
  command with an empty Docker configuration of its own, so that no login of the machine's is used.
- `make dataset-push`, the owner's: the work tree's lock the tag's (the version released); the version's layout the
  lock's (its digest, every blob's sha256); the owner's token asked for, not echoed, given to `crane auth login ghcr.io`
  in a temporary Docker configuration, removed when the push ends however it ends (Ctrl-C, SIGTERM and the terminal
  closed among them, each ending the step as Ctrl-C does); the layout pushed by crane to `<image>:<tag>`, the digest
  GHCR gives back the lock's.
- `make dataset-remote-check`, anyone's, after the push and once the image is public: the work tree's lock the tag's;
  with an empty Docker configuration, the tag on GHCR resolved to the lock's digest (`crane digest`) and the image read
  by its digest as in `dataset-check`.
- `make dataset-take-check`, anyone's, after the push, none of the version's files in the store: the work tree's lock
  the tag's; the version taken as a Nix taker takes it, by a taker's flake of its own (How it is checked).
- `check-datasets`, in `make check`, reading no network: every lock in `datasets/` well formed (its fields, the image
  `ghcr.io/zhifengzhang-sz/<dataset>`, its tag its version, each sha256 64 hex digits, its files in the order of their
  names), and, for a version whose tag is there, the same as the tag's.

## How it is checked

- `check-core` gains the program's tests, on the core's in-memory stream: the files, each line market's K-line in its
  canonical text read back by `raw.DecodeKLine` to the same base, the index from 0 across the files, the months by the
  zone; a copy dropped, the first taken; the faults, each named: an index not the next, a date not after the last, the
  topic short of the range, a range with no K-line, a flag refused; the layout: its index, manifest, config and layers
  as above, each layer's tar its one file, the config's `diff_ids` the layers' uncompressed sha256, made twice to the
  same bytes; the lock's fields.
- `make dataset`: the data set built twice to the same bytes.
- `make dataset-check`: the data set read back from the built image, every file the lock's.
- `make check` in the role `checker` after the last change, and `make products-check`.
- After the owner's push: `make dataset-remote-check`.
- After the owner's push, `make dataset-take-check`, anyone's, of a released version's lock as `dataset-remote-check`,
  by a taker's flake of its own in a temporary folder, pinning this repository's work tree (`git+file`) and following
  its nixpkgs: none of the version's files in the store before, else it stops naming them and the line that removes
  them (`nix-store --delete`); the version installed by the taker's own line, `nix run --inputs-from .
  platform#datasets -- DATASET VERSION`, in that folder, every file fetched from GHCR with no login to the store path
  its name and sha256 fix; the taker's package built, taking every file by `lib.dataset`: each file's store path that
  path, its sha256 and its K-lines the lock's, its link in `folder` that path; then installed again by the same line
  with the network cut (`HTTPS_PROXY` a closed port of 127.0.0.1): every file in the store, nothing fetched. It is the
  check of a release of `datasets` and of each data set's version after its push.

## The release

One commit: the product's version and the lock (`datasets/<product>.lock`, its digest and every file); its documents
(this design, `datasets/README.md` with its entry under Changes, `PRODUCTS.md`, `README.md`, `docs/designs/README.md`,
`docs/process.md`, `docs/decisions.md`); `docs/opening.md` and `docs/milestones.md`, saying what is true at the commit,
the owner's push and the check after it the next steps; tagged `<product>-<version>` and pushed with its tag by the
session. Nothing is committed after it. The owner's push and the image set public follow, then `make
dataset-remote-check` and `make dataset-take-check`. A push or check that fails changes nothing of the release: an
image that cannot be published as its lock says is never published, and the version that publishes is a new one, of a
commit of its own.

`datasets` is released as `practices.md`, 3, has it: one commit holding its number (`flake.nix`), its documents (this
design, `datasets/README.md` with its entry under Changes, `PRODUCTS.md`) and `docs/opening.md` and
`docs/milestones.md`; `make check`, `make products-check` and `make dataset-take-check` passed before it; tagged
`datasets-<version>` and pushed with its tag by the session.

## What is left

Each a step of its own:

- the installed files held from the store's garbage collection, by a root of the taker's;
- a version of a data set taken at a commit whose lock is another, every released version's lock kept;
- a version made on a schedule, as the record grows (a new version's range, its MINOR);
- a data set of another series, another cleaning's or another source's, once the platform has one;
- the image's digest in `make locks-check`, which asks `flake.lock`'s inputs and the project's own locks;
- a data set of the ongoing K-line, or of a forming one: the core's topics carry none;
- the charting consumer's K-line text (`consumers/chart/kline.go`), written there before the core had `raw.FromBase`,
  taken from the core.
