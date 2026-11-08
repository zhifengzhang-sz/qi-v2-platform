# The platform as an app: one image and one Compose app on GHCR, run by Docker alone (2026-11-09)

The platform given to a user without this folder as an app: one Compose application published on GHCR, which runs the
platform's programs from one image together with the Redpanda and the TimescaleDB they need, on a network and volumes
of its own. A user needs Docker and a Massive key of their own, runs one command, and reads one document, the app's; a
component of the user's is a container of its own on the app's network, in any language, built by a Dockerfile,
reading and writing the core's topics by the contract. How this repository makes and checks the app is the dev side,
apart from it: nothing of it (the flake, the Makefile, the test stack, `qi-v2-services`, this repository's layout) is
anything a user of the app knows. The same release gives the programs as a flake package to another qi-v2 project. This
is the design, made before its build; nothing of it is built yet. It rests on: qi-v2's `releaseImage` (`qi-v2-nix`,
`qi-v2/README.md` and `qi-v2/flake.nix`, at the commit `flake.lock` pins); nixpkgs at the commit `flake.lock` pins
(`dockerTools`, `buildGoModule` and its Go 1.26.10, skopeo 1.22.2, oras 1.3.3, `cacert`, `distribution` 3.1.1); Docker
Compose's own code at v5.7.0, the Compose specification and GitHub's documentation of its Container registry, read for
this design (`notes-compose-publish-2026-11-09.md` beside it: what an app is and how one runs; and Compose's
`pkg/remote/oci.go`, `internal/oci/resolver.go` and `cmd/compose/compose.go` at v5.7.0, read after, for its loader's
form and plain HTTP); oras' own code at v1.3.3 (`cmd/oras/root/cp.go`, `resolve.go`, `internal/option/remote.go`); the package `stack` 0.3.0 of `qi-v2-services` (its Compose files, for the versions); the programs and their
flags (`README.md`, The platform contract) and `make start` as it runs them (`dev/scripts/run.py`); the tutorial's
reader (`docs/tutorial/basic/`).

## What this design guarantees

1. **One command, Docker alone.** A user runs the app by one reference with Docker (its Compose v2.34.0 or later, whose
   loader of a published app is on by default) and a Massive key of their own, and nothing else: no stack to fetch, no
   file to download, no Nix, Go or clone of this repository, and no host port it takes. The app's document is all a
   user reads of how to run it; the topics it reads are the platform contract's, published beside it.
2. **The programs as they are.** The image holds the first path's programs as the flake package builds them, each run
   by its name with the flags the contract gives it, unchanged. The platform contract and the library are as they
   stand.
3. **One release, whole before it is published.** The flake package, the image and the app have one version. The
   release commit holds it, the documents, the image's digest and the app's, and is tagged, a tag for each product.
   The image and the app are built by Nix from the files they are made of alone, so the commit that records their
   digests does not change them, and building them again gives the same digests. The push to GHCR is the last step,
   the owner's, and is refused unless it pushes exactly those digests.
4. **Pinned and public.** The app names every image it runs by its digest (the platform's, Redpanda's, TimescaleDB's),
   and a user names the app by its digest; both are read from GHCR with no login, as every part of their build is.
5. **Apart from the dev stacks.** The app is a Compose project of its own, its own network and volumes, never
   `services` or `services-test`; it takes no host port, so it meets nothing of the stack or the test stack on a
   machine that holds them, and a run of the dev side (`make start`) and a run of the app share no topic, group or
   store.
6. **The key at run time, to the one program that reads it.** Massive's key is in no image, layer, label, build input,
   app or document: it is mounted read-only into acquisition's container when that container starts, from
   `~/.config/qi/massive_key`, and read only by the library's fetcher in `massive-reader`, as on the host; never
   printed.
7. **Ctrl-C reaches the program; nothing of a step is left.** Each program is its container's first process, so a
   stop's signal is the program's own; a container ends with every process in it; after a run, a check finds no
   container of the run's project and none of the check's own.
8. **A user's component, by Docker alone.** A component is a container of its own on the app's network, in any
   language, built by a Dockerfile, given the contract, the topics, the network's name and the broker's address on it;
   the tutorial's reader shows it, built by Docker alone.
9. **No trial on the data.** The app's check runs the app as a user would, on the real data, under a project of the
   check's own, wiped after; the stack, which holds the platform's data, is never touched.
10. **No login, nothing private.** Every part is a public repository's, nixpkgs', Docker Hub's, Redpanda's or GHCR's,
    read with no login; the app and its documents name no private project.

## The first path

A user runs the app; its first run makes one series' clean topic on the real data; a component of the user's reads
it. The app brings up Redpanda and TimescaleDB, then `provision` once, then the TimescaleDB writer, and acquisition on
`massive-spy-1m` as the tutorial's catalog has it (`-follow`). Acquisition's first run reads Massive's two years,
publishes them, and makes `market.clean.validation.v0-1-0.massive.SPY.M1` by its first reset, from the TimescaleDB
store's corrected raw, writing the store's clean series. Then the tutorial's reader, `clean-reader`, built by its
Dockerfile and run as a container of its own on the app's network, reads the clean topic from its start to its end.

The path's programs are five: `provision`; `acquisition`; `massive-reader` and `validation`, the catalog's plugins,
which acquisition runs; and `timescaledb-writer`, a container of its own and the command of acquisition's `-corrected`
and `-store`, which acquisition runs too. The path needs Redpanda and TimescaleDB and nothing else of the stack:
acquisition needs one store whose corrected raw a reset reads (`-corrected`, the TimescaleDB writer) and at least one
whose clean series a reset writes (`-store`); the TimescaleDB store is both, so the clean topic is made without
ClickHouse. `query` is not in the app: it names a stack by `-stack` alone, `localhost:19092` or `localhost:29092`, so it
reads no broker from inside a container (What is left).

## The app, as a user has it

**What a user runs.** With their key in `~/.config/qi/massive_key`, one line:

    docker compose -f oci://ghcr.io/zhifengzhang-sz/qi-v2-platform@sha256:<the app's digest> up -d -y

and `… down` to stop it, `… down -v` to wipe its data as well. For an app read from a registry, `up` lists the
variables it reads with their values, here `HOME` (the key's folder), and asks whether to go on, each time, unless `-y`
(Compose's `checksForRemoteStack`); the document gives each command as it runs with no question, every command of it
run at the build and its output given as it came. The project is `qi-v2-platform`, by the app's own `name`; no service
sets a `container_name`, so a project of another name (the check's) meets none of it.

**What it runs**, each service an image by its digest, on the project's default network, `qi-v2-platform_default`:

- `redpanda`: Redpanda at the version the pinned stack runs (`v26.2.6`), its Kafka listener `redpanda:9092` on the
  app's network alone (advertised so), in dev-container mode on one core as the stack runs it; its data in the volume
  `redpanda`; healthy by `rpk cluster health`.
- `timescaledb`: TimescaleDB at the version the pinned stack runs (`2.30.1-pg17`), on the app's network alone; its user
  and password the local stacks' (`postgres`, `password`, no secret: no host port reaches it); its data in the volume
  `timescaledb`; healthy by `pg_isready`.
- `provision`: `provision -brokers redpanda:9092`, once Redpanda is healthy, at each `up`, idempotent.
- `timescaledb-writer`: `timescaledb-writer -brokers redpanda:9092 -url postgres://postgres:password@timescaledb:5432/postgres`,
  once `provision` has ended with 0 and TimescaleDB is healthy.
- `acquisition`: `acquisition -dataset massive-spy-1m -catalog /etc/qi/catalog.yaml -brokers redpanda:9092 -corrected
  "timescaledb-writer -brokers redpanda:9092 -url …" -store "timescaledb-writer -brokers redpanda:9092 -url …"
  -follow`, once `provision` has ended with 0 and the writer has started: `run.py`'s acquisition, with the TimescaleDB
  store alone. Its catalog an inline config of the app, mounted at `/etc/qi/catalog.yaml`; Massive's pages in the
  volume `pages`, at `/work`; the key a secret.
- the programs' services `restart: "no"`, as `make start` restarts none (a program that ends on its own says why in its
  log, `docker compose logs`, and the next `up` starts it again), `stop_grace_period: 120s`, as `run.py` waits 120 s
  after SIGTERM; Redpanda's and TimescaleDB's `restart: unless-stopped`, as servers.

**The key.** A Compose secret from `${HOME}/.config/qi/massive_key`, given to acquisition's service alone, mounted
read-only at `/run/secrets/massive_key` when the container starts; `massive-reader`, acquisition's child, reads it there
through the library's fetcher (`-key`, in the catalog's command). The app names the key's path alone. The container's
user is 1000; a key that user cannot read stops the run at the fetcher, naming the file.

**The catalog**, inline in the app (`configs: catalog: content:`, the Compose specification's, plain Compose since
v2.23.1): `massive-spy-1m` as the tutorial's catalog has it, its commands the image's, `command: [massive-reader, -key,
/run/secrets/massive_key, -pages, /work/pages]` and `cleaning: [validation]`. Inline, since an app carries none of the
files beside it: a file of a published app is read from the user's machine, where there is none (Compose keeps a
`file:` path as written, and resolves a relative one in its own cache folder).

**State and data.** Acquisition keeps no state of its own (its state is the record, the topics); the writer's is its
group and its store. The app's data is in its three volumes, `redpanda` (the topics, each kept forever by its own
configuration, as `provision` sets it), `timescaledb` (the store) and `pages` (Massive's pages); `down` keeps them and the next `up` takes up where it
stopped; `down -v` wipes them. The pages' volume is made from the image's `/work`, owned by the user 1000. The logs are
Docker's.

**A component of the user's.** A container of its own on the app's network, `--network qi-v2-platform_default`, its
broker `redpanda:9092`, reading the core's topics by the contract and writing its own by the contract's rules; any
language with a Kafka client; built by a Dockerfile. The tutorial's reader is the example, its source a component's
source the app's document points to: `docs/tutorial/basic/clean-reader/Dockerfile` builds it as it stands, its code and
`go.mod` unchanged:

- a build stage on Docker Hub's `golang` at the shell's Go version, 1.26.10 today, pinned by its digest (Docker Hub
  holding no image of that version, the build stops and the design comes back); `GOTOOLCHAIN=local`; `CGO_ENABLED=0`;
- market's Go form from `qi-v2-contracts` at the commit `flake.lock` pins for `contracts` (`3354d2b…`, the tag
  `market-1.1.0`), read over HTTPS by the builder's Git source (`ADD` of the repository's Git URL at that commit; the
  build holds that the builder takes a commit, else it stops), its folder `market/go` put in place of the module
  `go.mod` requires at `v0.0.0` (`go mod edit -replace`, in the build alone), as `go.work` puts the flake's
  `market-go` there in the shell;
- its other modules, franz-go and its admin client, from the Go module proxy, held to its `go.sum`;
- a final stage of the program alone (`scratch`), run as the user 1000.

A user builds it by Docker alone from its Git URL at the release's tag, `docker build -t clean-reader
https://github.com/zhifengzhang-sz/qi-v2-platform.git#qi-v2-platform-0.1.0:docs/tutorial/basic/clean-reader`, with a
builder whose `ADD` takes a Git URL (BuildKit's, Docker's builder; the build names the least version it holds), and
runs it: `docker run --rm --network qi-v2-platform_default clean-reader -brokers redpanda:9092 -topic
market.clean.validation.v0-1-0.massive.SPY.M1`. It takes no package of the platform's, only the contracts and a Kafka
client (the basic level, 3).

**The app's document**, `app/README.md`: what a machine needs (Docker, its Compose v2.34.0 or later), the command, the
key, what runs, the volumes, how a component joins, the reader as the example, a run of every command with its output
as it came, and Changes. The topics and their schemas are not copied there: the document points to their one home, the
platform contract (`README.md`, The platform contract, and `core/README.md`) at the release's tag, the product a
component is built against. It speaks of the app alone: nothing of the flake, the Makefile, the test stack or
`qi-v2-services`.

## The dev side: the products and how they are made

**The flake package `platform`.** The five programs in one folder `bin/`, as `make build` puts the core's programs in
`bin/`, built by nixpkgs' `buildGoModule`, the flake's nixpkgs, whose Go is the shell's (`go` and `buildGoModule` both
`go_1_26`, 1.26.10):

- its source a file set of this repository: the Go modules the programs are built from (`core`, `lib`,
  `publishers/acquisition`, `consumers/timescaledb-writer`, `plugins/massive-reader`, `plugins/validation`), each one's
  `go.mod`, `go.sum`, its `.go` files but its tests, and the files it embeds (`lib/massive/aggregates.schema.json`,
  `consumers/timescaledb-writer/migrations/*.sql`); nothing else, so a document, the app, a test or another module
  changing changes none of its output;
- the Go workspace written in the build as `make go.work` writes it: the path's modules, and market's Go form at the
  flake's `market-go`;
- the modules the workspace takes, but market, fetched once by a fixed-output derivation (`go mod download`,
  `buildGoModule`'s `proxyVendor`) from the Go module proxy with no login, each held to its `go.sum`, the derivation's
  hash in `flake.nix`; the build itself with no network, as `make check`'s Go runs with `GOPROXY=off`;
- `CGO_ENABLED=0`, so each program is static; `-trimpath`, `buildGoModule`'s; no test in the build, the tests being
  `make check`'s;
- its run-time closure the package alone, held by the build (`nix path-info -r`): a store path a program still carries
  (Go writes a replaced module's folder, here `market-go`'s, into a program's build information) is taken out by
  nixpkgs' `removeReferencesTo`, so that neither `market-go` nor the source comes into the image;
- its version, the release's, in its name.

Another qi-v2 project takes it by a flake input at its tag, pinned by its own lock
(`github:zhifengzhang-sz/qi-v2-platform?ref=platform-0.1.0`), each program at `bin/<program>` of the package.

**The image `qi-v2-platform-programs`**, by `releaseImage pkgs { name = "qi-v2-platform-programs"; version; package =
platform; source = "https://github.com/zhifengzhang-sz/qi-v2-platform"; entrypoint; }`:
`ghcr.io/zhifengzhang-sz/qi-v2-platform-programs:<version>`, Ubuntu 24.04 under it by qi-v2's digest, run as the user
1000 in `/work`, its labels the repository and the version. Its entrypoint is

    /usr/bin/env PATH=<platform>/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
                 SSL_CERT_FILE=<cacert>/etc/ssl/certs/ca-bundle.crt

and a container's command follows it, a program's name and its flags. Each part, and why:

- **One image, a container's program its command.** Acquisition runs the catalog's plugins (`command`, `cleaning`) and
  the TimescaleDB writer (`-corrected`, `-store`) as processes of its own (`exec.CommandContext`, `acquire.go` and
  `clean.go`), so they are in its container however the programs are parted into images; one image holds them and the
  other programs alike, one digest a release.
- **`env`, Ubuntu's own** (`/usr/bin/env`, in the base). It sets the two variables and replaces itself by the program,
  so the program is the container's first process and a stop's SIGTERM, or Ctrl-C, reaches it; it brings nothing into
  the image.
- **`PATH`.** A bare name is found on it: the program a container's command names, and each program acquisition runs,
  which the catalog and acquisition's flags name by their bare names (acquisition keeps a bare name as it is,
  `catalog.go`, and Go's `exec` finds it on `PATH`). Ubuntu's own `PATH` follows, as the base gives it (`dockerTools`
  keeps a base's `Env`, nixpkgs' `stream_layered_image.py`, `overlay_base_config`), so that an image built on this one
  can add a program in `/usr/local/bin` and name it by its name (What is left, a user's cleaning).
- **`SSL_CERT_FILE`.** The base holds no certificate authority (Ubuntu's image at qi-v2's digest has no `/etc/ssl`), and
  `massive-reader` reaches Massive over HTTPS (`lib/massive`, `https://api.massive.com`). Go reads its roots from that
  file, nixpkgs' `cacert`, pinned by `flake.lock`, and acquisition's children take it from acquisition's environment.
- **Time zones.** The base has no zone data (no `/usr/share/zoneinfo/America/New_York`); acquisition, the one program
  of the path that loads a zone, embeds Go's (`time/tzdata`, `catalog.go`).
- **The closure.** Every store path the entrypoint names comes into the image with its closure, the image's config
  being one of `dockerTools`' roots (nixpkgs' `dockerTools`, `closureRoots`; qi-v2's README), and nothing else does:
  the package and `cacert`.

What a program is given: its flags, the container's command after its name, the contract's flags unchanged; the
catalog, the key and the pages as the app gives them (above); the addresses by flags (`-brokers`, `-url`). Of the
variables a program of the path reads, the container sets `SSL_CERT_FILE` alone: the TimescaleDB writer's driver, pgx,
reads `PG*` for what its `-url` leaves out (`PGPASSWORD` among them, its flag's help), and none is set, the URL giving
all. `HOME` (`/home/ubuntu`, the base's user 1000) is not used: each default that reads it, `massive-reader`'s `-key`
and `-pages`, is given in the catalog.

**The app `qi-v2-platform`.** Its one file, `app/compose.yaml` (the services, the inline catalog, the secret, the
volumes, `name: qi-v2-platform`), the platform's image named in it by `make image`'s digest, published as Compose
publishes an app (Compose's code, `internal/oci/push.go`): an OCI manifest of `artifactType`
`application/vnd.docker.compose.project`, its config the empty descriptor, its one layer the file, media type
`application/vnd.docker.compose.file+yaml`. The flake builds it as an OCI layout, `qi-v2-platform-oci`, by a Nix
derivation that writes those blobs and that manifest with nothing of time in it; `docker compose publish` is not used,
since it stamps the manifest with its time of publishing (`org.opencontainers.image.created`), so its digest would be
known only at the push. The app's digest is its manifest's. Compose's loader takes that form (its
`pkg/remote/oci.go`, `pullComposeFiles`): a manifest of the project's `artifactType` (or, of OCI 1.0, with Compose's
empty config type), each layer of the file's media type written to `compose.yaml`, no annotation needed for one
layer. Compose runs it by `-f oci://…@sha256:…` with no variable to set (its remote loader is on by default since
v2.34.0, and this machine's Compose is v5.7.0). Should GHCR refuse the OCI 1.1 form at the owner's push, the release
stops there and the design comes back, the OCI 1.0 form, which `docker compose publish` falls back to, the evident
change. Redpanda's and TimescaleDB's references in it are their tags at the versions the pinned stack's Compose file
names, with the digests their registries give for those tags, found with no login when the app is written:
`check-app`, which reads no network, holds the tags; `make app-test` holds each digest to its tag.

## The release

- **Its version and tags.** 0.1.0, the first: one number for the three products, written alike in `flake.nix` (once),
  `PRODUCTS.md`, `app/README.md` (Changes), `README.md` (Changes), the app's image reference and this design. A tag for
  each product, `<product>-<version>`, all on the release commit: `platform-0.1.0` (the flake package, which another
  project's flake input names), `qi-v2-platform-programs-0.1.0` (the image), `qi-v2-platform-0.1.0` (the app).
- **The digests before the push.** The flake gives the image as an OCI layout too, `qi-v2-platform-programs-oci`:
  skopeo (nixpkgs', 1.22.2), in a Nix build (`--insecure-policy`, the build's own copy of nothing but its input),
  copies the archive `releaseImage` gives to an OCI layout (the `oci` transport, which compresses each layer not
  compressed, gzip), so that the layout holds the manifest, the config and the layers as they are pushed; the image's
  digest is its manifest's. `make image` builds the package, the image and its layout, builds each again (`nix build
  --rebuild`: the same bytes, or it stops and the design comes back), and writes the image's digest into
  `app/compose.yaml`; then builds the app's layout, again twice, and writes the app's digest into `app/README.md`'s
  command. The image depends on the file set, `flake.lock`, `flake.nix`'s expressions and the version alone; the app
  on `app/compose.yaml` alone; neither on the commit (no `self.rev`, no date) or on another document; so writing the
  digests into the commit leaves both as they are, and the tags build them again to the same digests. This departs
  from qi-v2's README, whose release is `docker load`, then `docker push`: Docker compresses each layer as it pushes
  it, so its digest is known only from the push, where a layout's bytes, pushed as they are, give it before.
- **The release commit.** One commit: the version; the documents (this design, `PRODUCTS.md`, `app/README.md` with the
  app's digest and its entry under Changes, `README.md`); `app/compose.yaml` with the image's digest;
  `docs/opening.md` and `docs/milestones.md`, which say what is true at the commit: the release tagged, its image and
  app built and their digests, the owner's push to GHCR and the check after it the next steps, nothing yet on GHCR;
  the checker's pass on it (`make check`, `make app-test`). The commit and its tags are pushed to `origin` by the
  session. Nothing of the release is changed after the owner's push: the check's result is the next session's to
  record, in work of its own. A push or check that fails changes nothing of the release either: its cause is found,
  and an image or app at the tag that cannot be published as it is (GHCR holding another digest, or refusing the app's
  form) is never published; the tags stay, and the release that publishes is a new version, of a commit of its own.
- **The owner's push**, the last step, with a login of the owner's own, given whole:

      git switch --detach qi-v2-platform-0.1.0
      nix develop -c oras login ghcr.io --username zhifengzhang-sz   # a token that can write packages
      nix develop -c make release-push
      nix develop -c oras logout ghcr.io

  `make release-push` stops unless the work tree is clean and at the commit of the three tags of the flake's version;
  builds both layouts there; stops unless their digests are those of `app/compose.yaml` and `app/README.md`; then copies
  each layout to GHCR by oras (nixpkgs', 1.3.3; `oras cp --from-oci-layout <layout>:<tag> <reference>`, its own
  source at v1.3.3, `cmd/oras/root/cp.go`, which copies a manifest and its blobs as they are), the image first, to `ghcr.io/zhifengzhang-sz/qi-v2-platform-programs:<version>` and
  `ghcr.io/zhifengzhang-sz/qi-v2-platform:<version>`, and stops unless the digest GHCR gives back for each is the
  committed one. It reads no credential: oras uses its own login. A package first pushed to GHCR from a personal
  account is private, and a link to the repository makes it inherit the repository's permissions but not its
  visibility (GitHub's documentation); so the owner sets each of the two public once, on its page, a change GitHub
  allows no way back from.
- **The check after it, with no login**, `make release-check`, anyone's, with an empty Docker configuration (Anonymous
  pulls, below): each tag resolves to its committed digest, `ghcr.io/zhifengzhang-sz/qi-v2-platform:<version>` by
  `oras resolve` and `ghcr.io/zhifengzhang-sz/qi-v2-platform-programs:<version>` by `skopeo inspect --no-creds`, oras
  given an empty authentication file (`--registry-config`); `docker compose -f
  oci://ghcr.io/zhifengzhang-sz/qi-v2-platform@sha256:<the app's digest> config -q`, Compose's cache a temporary folder
  of its own (`XDG_CACHE_HOME`), since its loader keeps an app it has read under its digest and reads it from there
  after, so that the app is read from GHCR as a user's Compose reads it; then `docker pull` of each image the app
  names, by its digest, the platform's from GHCR, Redpanda's and TimescaleDB's from their registries. A package not
  public, or an app GHCR does not serve, fails it. GitHub's documentation names Docker and OCI images and says nothing of other OCI
  artifacts, so whether GHCR stores and serves the app is held first by the owner's push and this check.

## How it is checked

**Anonymous pulls.** Every pull a check makes is with an empty Docker configuration, `DOCKER_CONFIG` a temporary
folder, so that no login the machine holds is used; a per-user folder of Docker's plugins (`~/.docker/cli-plugins`),
where a machine has one, is linked into it, since Docker finds Compose and buildx there or in its system folder (on
this machine `/usr/libexec/docker/cli-plugins`).

**Before it is called done, `make app-test`**, run in the role `checker`, on the real data, as a user runs the app,
its steps one after another, each found ended after it. It uses Massive's key, as `make start STACK=test` does, so it
runs only when the brief of its run allows the key's use.

1. `make image`: the package, the image and both layouts built and built again, the same digests, written into
   `app/compose.yaml` and `app/README.md`;
2. a registry of the check's own: a container of nixpkgs' `distribution` 3.1.1 (its program `registry`), its image made
   by Nix for the check alone and never pushed, at `127.0.0.1:25000`, its storage inside the container; the image's
   layout copied to it by oras with its digest (`--to-plain-http`, the registry serving plain HTTP). Why: this
   machine's Docker keeps an image it loads with no registry digest (its image store `overlay2`), so an app that names
   the image by its digest runs only on an image pulled from a registry. Docker's daemon pulls an image from a registry
   on 127.0.0.0/8 without TLS by default;
3. the app's check form, `qi-v2-platform-check-oci`: the app's file with the image's registry `ghcr.io` given as
   `localhost:25000`, nothing else changed, built as the app is and copied to the check's registry; so the check runs
   the app's form, its file and the image by its digest from a registry, before GHCR is reached, and the app's own
   file names nothing of the check;
4. the app up by `docker compose --insecure-registry localhost:25000 -p qi-v2-platform-check -f
   oci://localhost:25000/zhifengzhang-sz/qi-v2-platform@sha256:<the check form's digest> up -d -y`, a project of the
   check's own, so that it meets no app a user runs on the machine; Compose's loader, not the daemon, reads the app,
   and over plain HTTP only from a registry its hidden flag `--insecure-registry` names, a flag for tests (its
   `internal/oci/resolver.go`, `cmd/compose/compose.go`); Compose's cache a temporary folder of the check's;
   Redpanda's and TimescaleDB's digests held each to its tag, resolved with no login (`skopeo inspect --no-creds`);
5. one wait for acquisition's line of the clean, `docker compose … logs -f acquisition` read until the line comes, then
   ended and found ended: the clean topic made by its first reset, none refused, the store's clean series written;
6. the reader built by its Dockerfile and run on the check's network, its K-lines into a file of a temporary folder;
   the topic read a second way, by `rpk topic consume` in the app's Redpanda (Redpanda's own client, in place of
   `query`, the platform's reader the basic level compares with, which is not in the app), each value's index, date and
   base fields written as the reader writes them, a copy dropped as the reader drops one (a value of a key and a date
   taken already, the first kept): the two files' md5 the same, and the K-lines and the copies each counted alike;
7. the app down with its volumes (`down -v`, the check's own project), the registry stopped; then nothing left: no
   container, network or volume of the project `qi-v2-platform-check`, none of the registry;
8. the key nowhere: no file of the image's layers named `massive_key` or under a home folder, its labels the two, its
   config's environment the base's `PATH` alone, the package's closure itself (`nix path-info -r`), and the app's file
   naming the key's path alone; the key itself is not read for it.

Ctrl-C at any step reaches the step's program and ends the check: its project taken down with its volumes (`down -v`),
its registry stopped, then step 7's check that nothing is left. The stack and the test stack are not touched by it.

**`make check` gains `check-app`**, run by `make verify` on `app/`, the Dockerfile, `flake.nix` and `flake.lock` too;
it builds nothing and reads no network: `app/compose.yaml` read by `docker compose config -q`; no service with a
`container_name`; its Redpanda and TimescaleDB at the tags the pinned stack's Compose file names (from the package
`stack` in the Nix store), each with a digest, which `make app-test` holds to its tag; its image reference
at the flake's version, with a digest; `app/README.md`'s command naming a digest; the Dockerfile's `golang` at the
shell's Go (`go env GOVERSION`), and its market at `flake.lock`'s commit of `contracts`. So a newer stack, nixpkgs' Go or
contracts taken into `flake.lock` stops at the check until the app or the Dockerfile follows.

**After the owner's push**, `make release-check` (The release).

## What changes, at the build

- `flake.nix`: the packages `platform`, `qi-v2-platform-programs` (`releaseImage`'s archive, for `docker load` by a
  project that takes it), `qi-v2-platform-programs-oci`, `qi-v2-platform-oci`, `qi-v2-platform-check-oci` and the
  check's registry image; oras and skopeo in the shell's packages; the modules' hash. `flake.lock`: unchanged (qi-v2's
  `releaseImage` at its pinned commit; skopeo, oras, `cacert` and `distribution` the pinned nixpkgs').
- `app/`: `compose.yaml` and `README.md`, the app's document (above).
- `docs/tutorial/basic/clean-reader/Dockerfile`, and a line of the basic level and of `docs/tutorial/README.md`
  naming it: the reader built by Docker alone and run on the app's network. The tutorial stays the dev side's, on the
  repository's shell and the test stack.
- `PRODUCTS.md`: in Products, `platform` (flake package, its tag `platform-<version>`, Doc `README.md`),
  `qi-v2-platform-programs` (image, `qi-v2-platform-programs-<version>`, Doc `README.md`) and `qi-v2-platform` (file,
  the Compose app, `qi-v2-platform-<version>`, Doc `app/README.md`), 0.1.0; Responsibility restated: the
  infrastructure is `qi-v2-services`', and the platform's app runs the Redpanda and TimescaleDB it needs, at the
  versions the pinned stack runs. Takes unchanged: the kit's check takes a row only for a flake input or a lock file,
  so Docker Hub's `golang` and the app's Redpanda and TimescaleDB images, each pinned by digest in its file, are named
  in `README.md`.
- `Makefile`: `image`, `app-test`, `release-push`, `release-check`, and `check-app` in `check`; their steps in
  `dev/scripts/app.py`; `dev/scripts/verify.py`'s table.
- `README.md`: the app, by its document; the flake package and the image for another project, and Changes; Dependencies
  and layout, `app/`, Docker Hub's `golang` and the app's images. The platform contract is unchanged: the image and the
  app are products (`PRODUCTS.md`), the programs and their flags in them the contract's as they stand.
- `docs/process.md` (Another project: what the platform publishes; The machine: the owner's GHCR login for the push;
  The files: `app/`), `docs/impl/engineering/dev.environment.md` (oras and skopeo; the app's check),
  `docs/impl/engineering/go.toolchain.md` (the programs built by Nix).
- `agents/rules.md`: the dependencies and the tools, restated for the Dockerfile and the app's images
  (`docs/decisions.md`, Open).

Of the design before this one, given up: the Compose file run beside the test stack, which joined its network, and
the stack taken from `qi-v2-services`' Compose files at its tag; with them, the guard against the host's `make start`
and the image on one stack, which the app no longer needs, sharing its stack with no other run (each project its own
broker and store), the dev side's runs kept apart by `run.py`'s pids as they stand. Lost with `query` not in the app:
the check's comparison with the platform's own reader; Redpanda's client takes its place.

## What is left

Each a step of its own:

- `query` in the image and the app, with a flag naming the brokers, so that a user reads the topics' questions from a
  container; a host port of the app's broker, for a component run on the host;
- `clickhouse-writer` and `printer` in the package and the image, and ClickHouse in the app, with the ClickHouse
  store's clean series written at a reset; Redpanda's Console in the app;
- a catalog of the user's own (other datasets, a cleaning or a source plugin of their own in an image built on the
  platform's, its program in `/usr/local/bin`): the app's catalog is its own; the tutorial's advanced level as an app,
  and its medium level's follower in a container;
- the dev side's stack, which holds the platform's data, run by the image: `make start` by the image;
- a password of the user's own for the app's TimescaleDB, where the app's is the local stacks';
- the app's Redpanda, TimescaleDB and `golang` digests in `make locks-check`, which asks `flake.lock`'s inputs and the
  project's own locks; market's Go form as a Go module version, which the library taken outside this repository needs
  too (milestone 2);
- an image and an app for a system other than x86_64-linux (qi-v2's Ubuntu is pinned for it alone);
- CI, building and checking the app (`docs/decisions.md`, Open, CI in the flake's shell);
- the flake package's programs as apps (`nix run`);
- a host whose user is not 1000, whose key the container's user 1000 reads.
