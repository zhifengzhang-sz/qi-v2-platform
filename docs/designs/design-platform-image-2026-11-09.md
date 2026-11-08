# The platform as an image: its programs on GHCR, run beside the services stack (2026-11-09)

The platform's programs given to a taker without this folder, in the two products of one release: a flake package,
which another qi-v2 project takes by a flake input pinned by its lock, and one OCI image made from it by Nix, public on
GHCR and pinned by digest, which a user outside runs with Docker alone beside `qi-v2-services`' stack. What runs the
image beside the stack is the platform's own, given with it: a Compose file and its catalog. A user's component is a
container of its own beside it, in any language, built by a Dockerfile, reading and writing the core's topics by the
contract. This is the design, made before its build; nothing of it is built yet. It rests on: qi-v2's `releaseImage`
(`qi-v2-nix`, `qi-v2/README.md` and `qi-v2/flake.nix`, at the commit `flake.lock` pins); nixpkgs at the commit
`flake.lock` pins (`dockerTools`, `buildGoModule` and its Go 1.26.10, skopeo 1.22.2, `cacert`, `distribution` 3.1.1);
the package `stack` 0.3.0 of `qi-v2-services` (its README and its Compose files); the programs and their flags
(`README.md`, The platform contract) and `make start` as it runs them (`dev/scripts/run.py`); the tutorial's reader
(`docs/tutorial/basic/`).

## What this design guarantees

1. **The programs as they are.** The image holds the first path's programs as the flake package builds them, each run
   by its name with the flags the contract gives it, unchanged: a container's command is a program and its flags. The
   platform contract and the library are as they stand.
2. **One release, whole before it is published.** The flake package, the image and the Compose file have one version.
   The release commit holds it, the documents and the image's digest, and is tagged, a tag for each product. The image
   is built by Nix from the sources it is made of alone, so the commit that records its digest does not change it, and
   building it again gives the same digest. The push to GHCR is the last step, the owner's, and is refused unless it
   pushes exactly that digest.
3. **Pinned and public.** Every place that runs the image names it by its digest; it is read from GHCR with no login,
   as every part of its build is (nixpkgs, qi-v2's Ubuntu base by its digest, the Go modules by their `go.sum`).
4. **Docker alone, outside.** A user outside runs the stack, the image and a component of its own with Docker alone:
   the stack from `qi-v2-services`' Compose files at the tag `flake.lock` pins, the image from GHCR, the component built
   by its Dockerfile; no Nix, Go or clone of this repository. Nix builds the image, here, at the release.
5. **Beside the stack, never in it.** The platform's containers are a Compose project of their own, never `services`
   or `services-test`, joining the stack's network; the project has no volume, and nothing of the stack's project is
   started, stopped or removed by it: no `--remove-orphans`, no `-v`. The platform's own targets never run the image
   and the host's programs on one stack at once.
6. **The key at run time, to the one program that reads it.** Massive's key is in no image, layer, label, build input
   or document: it is mounted read-only into acquisition's container when that container starts, from
   `~/.config/qi/massive_key`, and read only by the library's fetcher in `massive-reader`, as on the host; never
   printed.
7. **Ctrl-C reaches the program; nothing of a step is left.** The program is its container's first process, so a stop's
   signal is the program's own; a container ends with every process in it; after a run, a check finds no container of
   the run's project and none of the check's own.
8. **A user's component, by Docker alone.** A component is a container of its own beside the platform's, in any
   language, built by a Dockerfile, given the contract, the topics, the stack's network and the broker's address on it;
   the tutorial's reader shows it, built from this repository at the release's tag by Docker alone.
9. **Every trial on the test stack.** The checks run the image on the test stack, Massive's pages in a temporary folder
   of the run's own, the test stack wiped after.
10. **No login, nothing private.** Every part is a public repository's, nixpkgs', Docker Hub's or GHCR's, read with no
    login; the image and its documents name no private project.

## The first path

One series' clean topic made from the image on the test stack, on the real data, and one component reading it. The
test stack up; the platform's Compose project on it from the image: `provision` once, then the TimescaleDB writer, and
acquisition on `massive-spy-1m` as the tutorial's catalog has it (`-follow`), its pages in the run's folder.
Acquisition's first run reads Massive's two years, publishes them, and makes
`market.clean.validation.v0-1-0.massive.SPY.M1` by its first reset, from the TimescaleDB store's corrected raw, writing
the store's clean series. Then the tutorial's reader, `clean-reader`, built by its Dockerfile and run as a container of
its own on the test stack's network, reads the clean topic from its start to its end, and its K-lines are, field for
field, those `query` gives of the topic (`docs/tutorial/basic/README.md`, 3, the md5 of both).

The path's programs are five: `provision`; `acquisition`; `massive-reader` and `validation`, the catalog's plugins,
which acquisition runs; and `timescaledb-writer`, a container of its own and the command of acquisition's `-corrected`
and `-store`, which acquisition runs too. The ClickHouse writer is not among them: acquisition needs one store whose
corrected raw a reset reads (`-corrected`, the TimescaleDB writer) and at least one whose clean series a reset writes
(`-store`); the TimescaleDB store is both, so the clean topic is made without ClickHouse. `query` is not either: it
names a stack by `-stack` alone, `localhost:19092` or `localhost:29092`, which inside a container is the container
itself, so it reads no stack from one without a flag of its own (What is left). Everything else is left (What is left).

## The two products

**The flake package `platform`.** The five programs in one folder `bin/`, as `make build` puts the core's programs in
`bin/`, built by nixpkgs' `buildGoModule`, the flake's nixpkgs, whose Go is the shell's (`go` and `buildGoModule` both
`go_1_26`, 1.26.10):

- its source a file set of this repository: the Go modules the programs are built from (`core`, `lib`,
  `publishers/acquisition`, `consumers/timescaledb-writer`, `plugins/massive-reader`, `plugins/validation`), each one's
  `go.mod`, `go.sum`, its `.go` files but its tests, and the files it embeds (`lib/massive/aggregates.schema.json`,
  `consumers/timescaledb-writer/migrations/*.sql`); nothing else, so a document, the Compose file, a test or another
  module changing changes none of its output;
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

**The image `qi-v2-platform`**, by `releaseImage pkgs { name = "qi-v2-platform"; version; package = platform; source =
"https://github.com/zhifengzhang-sz/qi-v2-platform"; entrypoint; }`, the flake's package `qi-v2-platform`:
`ghcr.io/zhifengzhang-sz/qi-v2-platform:<version>`, Ubuntu 24.04 under it by qi-v2's digest, run as the user 1000 in
`/work`, its labels the repository and the version. Its entrypoint is

    /usr/bin/env PATH=<platform>/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
                 SSL_CERT_FILE=<cacert>/etc/ssl/certs/ca-bundle.crt

and a container's command follows it, a program's name and its flags: `docker run … qi-v2-platform@sha256:<digest>
acquisition -dataset massive-spy-1m …`. Each part, and why:

- **One image, a container's program its command.** Acquisition runs the catalog's plugins (`command`, `cleaning`) and
  the TimescaleDB writer (`-corrected`, `-store`) as processes of its own (`exec.CommandContext`, `acquire.go` and
  `clean.go`), so they are in its container however the programs are parted into images; one image holds them and the
  other programs alike, and a release has one image, one digest to compute, push and check.
- **`env`, Ubuntu's own** (`/usr/bin/env`, in the base). It sets the two variables and replaces itself by the program,
  so the program is the container's first process and a stop's SIGTERM, or Ctrl-C, reaches it; it brings nothing into
  the image. Given no command, it prints its environment, the two variables and Docker's own, and ends.
- **`PATH`.** A bare name is found on it: the program a container's command names, and each program acquisition runs,
  which the image's catalog and acquisition's flags name by their bare names (acquisition keeps a bare name as it is,
  `catalog.go`, and Go's `exec` finds it on `PATH`). Ubuntu's own `PATH` follows the platform's folder, as the base
  gives it (the base's `PATH` is `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`; `dockerTools` keeps a
  base's `Env`, nixpkgs' `stream_layered_image.py`, `overlay_base_config`), so that an image built on this one can add a
  program in `/usr/local/bin` and name it by its name (What is left, a user's cleaning).
- **`SSL_CERT_FILE`.** The base holds no certificate authority (Ubuntu's image at qi-v2's digest has no `/etc/ssl`), and
  `massive-reader` reaches Massive over HTTPS (`lib/massive`, `https://api.massive.com`). Go reads its roots from that
  file, nixpkgs' `cacert`, pinned by `flake.lock`, and acquisition's children take it from acquisition's environment.
- **Time zones.** The base has no zone data (no `/usr/share/zoneinfo/America/New_York`); acquisition, the one program
  of the path that loads a zone, embeds Go's (`time/tzdata`, `catalog.go`).
- **The closure.** Every store path the entrypoint names comes into the image with its closure, the image's config
  being one of `dockerTools`' roots (nixpkgs' `dockerTools`, `closureRoots`; qi-v2's README), and nothing else does:
  the package and `cacert`.

What a program is given, and how: its flags, the container's command after its name, the contract's flags unchanged;
the catalog, a file mounted read-only (`-catalog`); Massive's key, a file mounted read-only when the container starts
(`-key`, in the catalog's command); Massive's pages, a folder of the taker's mounted at `/work` (`-pages`, in the
catalog's command); the stack's addresses, by flags (`-brokers`, `-url`). Of the variables a program of the path reads,
the container sets `SSL_CERT_FILE` alone: the TimescaleDB writer's driver, pgx, reads `PG*` for what its `-url` leaves
out (`PGPASSWORD` among them, its flag's help), and none is set, the URL giving all. `HOME` (`/home/ubuntu`, the base's
user 1000) is not used: each default that reads it, `massive-reader`'s `-key` and `-pages`, is given in the catalog.

## The release

- **Its version and tags.** 0.1.0, the first: one number for both products and the Compose file, written alike in
  `flake.nix` (once, for both), `PRODUCTS.md`, `deploy/README.md` (its Changes), the Compose file's image reference and
  this design. A tag for each product, `<product>-<version>`, both on the release commit: `platform-0.1.0`, the flake
  package's, which another project's flake input names; `qi-v2-platform-0.1.0`, the image's.
- **The image's digest before the push.** The flake gives the image as an OCI layout too, `qi-v2-platform-oci`: skopeo
  (nixpkgs', 1.22.2), in a Nix build (`--insecure-policy`, the build's own copy of nothing but its input), copies the
  archive `releaseImage` gives to an OCI layout (the `oci` transport, which compresses each layer not compressed, gzip),
  so that the layout holds the manifest, the config and the layers as they are pushed. The image's digest is its
  manifest's, in the layout's `index.json`. `make image` builds the package, the image and the layout, builds each
  again (`nix build --rebuild`: the same bytes, or it stops and the design comes back), and writes the digest into the
  Compose file's image reference. The image depends on the file set, `flake.lock`, `flake.nix`'s expressions and the
  version alone, never on the commit (no `self.rev`, no date: `dockerTools`' times are 1970) or on a document; so
  writing the digest into the commit leaves the image as it is, and the tag builds it again to the same digest. This
  departs from qi-v2's README, whose release is `docker load`, then `docker push`: Docker compresses each layer as it
  pushes it, so its digest is known only from the push, where the layout's bytes, pushed as they are, give it before.
- **The release commit.** One commit: the version; the documents (this design, `PRODUCTS.md`, `deploy/README.md` with
  its entry under Changes, `README.md`, the tutorial); the Compose file with the image's digest; `docs/opening.md` and
  `docs/milestones.md`, which say what is true at the commit: the release tagged, its image built and its digest, the
  owner's push to GHCR and the check after it the next steps, the image not yet on GHCR; the checker's pass on it
  (`make check`, `make image-test`). The commit and its tags are pushed to `origin` by the session. Nothing of the
  release is changed after the owner's push: the check's result is the next session's to record, in work of its own.
  A push or check that fails changes nothing of the release either: its cause is found, and an image at the tag that
  cannot be published as it is (GHCR holding another digest) is never published; the tags stay, and the release
  that publishes is a new version, of a commit of its own.
- **The owner's push**, the last step, with a login of the owner's own, given whole:

      git switch --detach qi-v2-platform-0.1.0
      nix develop -c skopeo login ghcr.io --username zhifengzhang-sz   # a token that can write packages
      nix develop -c make release-push
      nix develop -c skopeo logout ghcr.io

  `make release-push` stops unless the work tree is clean and at the commit of both tags of the flake's version;
  builds `qi-v2-platform-oci` there; stops unless its digest is the Compose file's; then copies the layout to
  `docker://ghcr.io/zhifengzhang-sz/qi-v2-platform:<version>` (`skopeo copy --policy <skopeo>/etc/containers/default-policy.json
  --preserve-digests --digestfile …`, nixpkgs' skopeo keeping its policy in its own package), and stops unless the
  digest GHCR took is the Compose file's. It reads no credential: skopeo uses its own login. The first push makes the
  package `qi-v2-platform` on GHCR; whether it is public is the package's own setting on GitHub (qi-v2's README), which
  the owner sets to public once, on the package's page.
- **The check after it, with no login**, `make release-check`, anyone's: `skopeo inspect --no-creds` of
  `ghcr.io/zhifengzhang-sz/qi-v2-platform:<version>`, whose digest is the Compose file's at the tag; then `docker pull`
  of the image by that digest with an empty Docker configuration (Anonymous pulls, below); Docker holds the manifest it
  pulls to its digest. A package not public fails both.

## What runs it beside the stack

`deploy/compose.yaml`, a Compose file, and `deploy/catalog.yaml`, its catalog: the platform's own, given with the image
at the release's tag (read over HTTPS, the repository public), with `deploy/README.md`, the two products' document.

- **The project** `platform-test` (the file's `name`), beside the test stack's `services-test`; never `services`, which
  holds containers and volumes that are not the stack's.
- **The network.** The test stack's, `services-test_qitest-network` (`qi-v2-services`' README, Test stack), joined as
  `external`. A container reaches the stack on it alone: Redpanda advertises its external listener at `localhost:29092`,
  which inside a container is the container itself, and its internal one at `redpanda:9092`, a name on that network
  (the test stack's Compose file). So the addresses are the stack's services by their names: `-brokers redpanda:9092`;
  `-url postgres://postgres:password@timescaledb:5432/postgres` (the local stacks' password, none). With the test stack
  not up, the network is not found and `up` stops, naming it.
- **The services**, each of `image: ghcr.io/zhifengzhang-sz/qi-v2-platform:0.1.0@sha256:<digest>`, one reference by a
  YAML anchor, its one home (its registry `${QI_REGISTRY:-ghcr.io/zhifengzhang-sz}`, which the check sets to its own,
  How it is checked):
  - `provision`: `provision -brokers redpanda:9092`, at each `up`, idempotent (`core/cmd/provision`);
  - `timescaledb-writer`: `timescaledb-writer -brokers redpanda:9092 -url …`, once `provision` has ended with 0
    (`service_completed_successfully`);
  - `acquisition`: `acquisition -dataset massive-spy-1m -catalog /etc/qi/catalog.yaml -brokers redpanda:9092
    -corrected "timescaledb-writer -brokers redpanda:9092 -url …" -store "timescaledb-writer -brokers redpanda:9092
    -url …" -follow`, once `provision` has ended with 0 and the writer has started: `run.py`'s acquisition, with the
    TimescaleDB store alone;
  - each `restart: "no"`, as `make start` restarts none (a program that ends on its own says why in its log, `docker
    compose logs`, and the next `up` starts it again); `stop_grace_period: 120s`, as `run.py` waits 120 s after SIGTERM.
- **The catalog** `deploy/catalog.yaml`: `massive-spy-1m` as the tutorial's catalog has it, its commands the image's,
  `command: [massive-reader, -key, /run/secrets/massive_key, -pages, /work/pages]` and `cleaning: [validation]`;
  mounted read-only at `/etc/qi/catalog.yaml` in acquisition's container.
- **State and pages.** Acquisition keeps no state of its own (its state is the record, the topics); the writer's is its
  group on the stack and its store. Massive's pages are the one thing a container keeps: in the taker's folder,
  `QI_WORK`, mounted at `/work` in acquisition's container (`${QI_WORK:?…}`: unset, `up` stops, naming it), the pages at
  `/work/pages`; on the test stack a temporary folder of the run's own (`mktemp -d`), never the stack's pages. The logs
  are Docker's. The project has no volume, so `docker compose down` leaves nothing of it.
- **Massive's key.** A Compose secret from `${HOME}/.config/qi/massive_key`, given to acquisition's service alone,
  mounted read-only at `/run/secrets/massive_key` when the container starts; `massive-reader`, acquisition's child,
  reads it there through the library's fetcher (`-key`). It is read by the container's user, 1000, as the host's user is
  1000 on this machine; on another machine, a key the user 1000 cannot read stops the run at the fetcher, naming the
  file. Nothing of the image, its build or the Compose file holds the key: the Compose file names its path alone.
- **Up and down.** `docker compose -f deploy/compose.yaml up -d`, `QI_WORK` set, the test stack up first; `docker
  compose -f deploy/compose.yaml down` stops each program by SIGTERM and removes the project's containers and its own
  default network; never `--remove-orphans`, never `-v`. Since the project's containers hold the test stack's network,
  `make stack-test-stop` and `make stack-test-down` take the project `platform-test` down first, as they stop the host's
  components first now.
- **One stack, one run.** The host's `make start` and the image both take the groups `acquisition` and
  `timescaledb-writer`, and two acquisitions would publish one series twice; so the platform's targets refuse the one
  while the other runs on that stack: `run.py start` while a container of the project `platform-test` is there,
  `make image-test` while `run.py` has a component running on the test stack. A user who runs the Compose file by hand
  is told so in `deploy/README.md`; a guard in acquisition itself, for any two runs, is left.

## The stack, outside

An external user runs the stack from `qi-v2-services`' own Compose files at the tag `flake.lock` pins for `services`,
`stack-0.3.0`: `services/docker-compose.test.yml` and, beside it, `services/config/redpanda-console.yml`, read over
HTTPS from its public repository, then `docker compose -f services/docker-compose.test.yml up -d --wait`, the project
`services-test` by the file's own `name`, and `down -v` to wipe it, as `qi-v2-services`' README says. They are the files
the package `stack` runs: the package is `docker compose --file <its Compose file>` and nothing more (its `bin/stack`
and `bin/stack-test`), so the stack is the same, and Nix is not needed for it. This repository's own targets keep the
package (`make stack-up`, `make stack-test-up`, the checks). `check-image` holds the tag `deploy/README.md` and the
tutorial's page name to `flake.lock`'s ref of `services`. The Compose files as a product of their own are
`qi-v2-services`' to give (`docs/decisions.md`, Open).

## A user's component

What a component is given: the platform contract (`README.md`) and the topics' schemas (`core/README.md`); the topics'
names; the stack's network, on the test stack `services-test_qitest-network`, and the broker's address on it,
`redpanda:9092`. What it needs on the machine: Docker, with Compose and its builder, BuildKit, as Docker ships them, and
a way to read a file over HTTPS (curl) for the Compose files of the stack and of the platform.

A component is an image of the user's, built by a Dockerfile, in any language with a Kafka client, run as a container
of its own on the stack's network, beside the platform's project (`docker run --network …`, or a Compose file of its
own joining the network), reading the core's topics and writing its own by the contract's rules.

**The first path's component: the tutorial's reader.** `docs/tutorial/basic/clean-reader/Dockerfile` builds
`clean-reader` as it stands, its code and its `go.mod` unchanged:

- a build stage on Docker Hub's `golang` at the shell's Go version, 1.26.10 today, pinned by its digest (Docker Hub
  holding no image of that version, the build stops and the design comes back); `GOTOOLCHAIN=local`, so that Go
  fetches no other toolchain; `CGO_ENABLED=0`;
- market's Go form from `qi-v2-contracts` at the commit `flake.lock` pins for `contracts` (`3354d2b…`, the tag
  `market-1.1.0`), read over HTTPS by the builder's Git source (`ADD` of the repository's Git URL at that commit; the
  build holds that the builder takes a commit, else it stops), its folder `market/go` put in place of the module
  `go.mod` requires at `v0.0.0` (`go mod edit -replace`, in the build alone), as `go.work` puts the flake's
  `market-go` there in the shell;
- its other modules, franz-go and its admin client, from the Go module proxy, held to its `go.sum`;
- a final stage of the program alone (`scratch`), run as the user 1000.

It is built from the repository at the release's tag by Docker alone, `docker build -t clean-reader
https://github.com/zhifengzhang-sz/qi-v2-platform.git#qi-v2-platform-0.1.0:docs/tutorial/basic/clean-reader`, or from
a clone, and run beside the platform: `docker run --rm --network services-test_qitest-network clean-reader -brokers
redpanda:9092 -topic market.clean.validation.v0-1-0.massive.SPY.M1`. It shows a user's component because it takes no
package of the platform's, only the contracts and a Kafka client (the basic level, 3), is built by Docker alone, and
reads the clean topic from a container of its own over the stack's network, by the contract.

## How it is checked

**Anonymous pulls.** Every pull a check makes (Docker Hub's `golang` and Ubuntu's, the check's registry, GHCR) is made
with an empty Docker configuration, `DOCKER_CONFIG` a temporary folder, so that no login the machine holds is used; a
per-user folder of Docker's plugins (`~/.docker/cli-plugins`), where a machine has one, is linked into it, since
Docker finds Compose and buildx there or in its system folder (on this machine `/usr/libexec/docker/cli-plugins`).

**Before it is called done, `make image-test`**, run in the role `checker`, on the test stack, on the real data, its
steps one after another, each found ended after it. It uses Massive's key, as `make start STACK=test` does, so it runs
only when the brief of its run allows the key's use.

1. the test stack up, empty (`make stack-test-up`), no component of `run.py`'s running on it;
2. `make image`: the package, the image and the layout built and built again, the same digest, written into the
   Compose file;
3. a registry of the check's own: a container of nixpkgs' `distribution` 3.1.1 (its program `registry`), its image made
   by Nix for the check alone and never pushed, at `127.0.0.1:25000`, its storage inside the container; the layout
   copied to it by skopeo (`--preserve-digests --digestfile`, and `--dest-tls-verify=false`, the registry serving plain
   HTTP), the digest it took the Compose file's. Why: this machine's Docker keeps an image it loads with no registry
   digest (its image store `overlay2`), so a Compose file that names the image by its digest runs only on an image
   pulled from a registry; with this one, the release's Compose file runs as the release holds it, before GHCR is
   reached. Docker takes a registry on 127.0.0.0/8 without TLS by default;
4. the platform's project up from the Compose file, `QI_REGISTRY=localhost:25000/zhifengzhang-sz`, `QI_WORK` a
   temporary folder;
5. one wait for acquisition's line of the clean, `docker compose logs -f acquisition` read until the line comes, then
   ended and found ended: the clean topic made by its first reset, none refused, the store's clean series written;
6. the reader built by its Dockerfile and run on the network, its K-lines into a file of the run's folder: their md5
   is the md5 of `bin/query bars -stack test -topic …`'s (the basic level, 3; `query` built by `make build`), their
   count the topic's;
7. the project down, the registry stopped, `make stack-test-down`; then nothing left: no container of the project
   `platform-test`, none of the registry, no `qitest-*`;
8. the key nowhere: no file of the image's layers named `massive_key` or under a home folder, its labels the two, its
   config's environment the base's `PATH` alone, and the package's closure itself (`nix path-info -r`); the key itself
   is not read for it.

Ctrl-C at any step reaches the step's program and ends the check, its project, its registry and the test stack taken
down.

**`make check` gains `check-image`**, run by `make verify` on `deploy/`, the Dockerfile, `flake.nix` and `flake.lock`
too; it builds nothing: the Compose file read by `docker compose config -q`; its network the test stack's of the pinned
`stack` (the project's and the network's names, from its Compose file in the Nix store); its image reference at the
flake's version, with a digest; the Dockerfile's `golang` at the shell's Go (`go env GOVERSION`), and its market at
`flake.lock`'s commit of `contracts`; the stack's tag in `deploy/README.md` and the tutorial's page at `flake.lock`'s
ref of `services`. So a newer stack, nixpkgs' Go or contracts taken into `flake.lock` stops at the check until the
Compose file, the Dockerfile or the documents follow.

**After the owner's push**, `make release-check` (The release).

## What changes, at the build

- `flake.nix`: the packages `platform`, `qi-v2-platform` (`releaseImage`'s archive, for `docker load` by a project that
  takes it), `qi-v2-platform-oci` (the layout) and the check's registry image; skopeo in the shell's packages; the
  modules' hash. `flake.lock`: unchanged (qi-v2's `releaseImage` is at its pinned commit; skopeo, `cacert` and
  `distribution` are the pinned nixpkgs').
- `deploy/`: `compose.yaml`, `catalog.yaml`, `README.md` (the two products; the stack, the image and a component run by
  Docker alone beside the test stack, the key, the folder, one run on a stack; the push and the check; Changes).
- `docs/tutorial/basic/clean-reader/Dockerfile`; a page `docs/tutorial/image/README.md` (the external user's way: the
  test stack from `qi-v2-services`' Compose files, the platform from the image, the reader built by its Dockerfile and
  run beside it, every command run on the test stack and every output as it came), and its row in
  `docs/tutorial/README.md`.
- `PRODUCTS.md`: in Products, `platform` (flake package, its tag `platform-<version>`) and `qi-v2-platform` (image, its
  tag `qi-v2-platform-<version>`), 0.1.0, their Doc `deploy/README.md`. Takes unchanged: the kit's check takes a row only
  for a flake input or a lock file, so Docker Hub's `golang`, pinned by the Dockerfile, is named in `README.md`.
- `Makefile`: `image`, `image-test`, `release-push`, `release-check`, and `check-image` in `check`; `stack-test-stop`
  and `stack-test-down` taking the project `platform-test` down first; their steps in `dev/scripts/image.py`; `run.py`
  refusing a start beside the image's project; `dev/scripts/verify.py`'s table.
- `README.md`: Running it, the image and the stack by Docker alone; Dependencies and layout, `deploy/` and Docker Hub's
  `golang` for the Dockerfile. The platform contract is unchanged: the image is a product (`PRODUCTS.md`), the programs
  and their flags in it the contract's as they stand.
- `docs/process.md` (Another project: what the platform publishes; The machine: the owner's GHCR login for the push;
  The files: `deploy/`), `docs/impl/engineering/dev.environment.md` (skopeo; the image on the test stack),
  `docs/impl/engineering/go.toolchain.md` (the programs built by Nix).
- `agents/rules.md`: the dependencies and the tools, restated for the Dockerfile (`docs/decisions.md`, Open).

## What is left

Each a step of its own:

- the image beside the stack, the platform's data: a project of its own on `services_qicore-network`, the stack's
  folder of pages; and `make start` by the image;
- `clickhouse-writer` and `printer` in the package and the image, and the ClickHouse store's clean series written at a
  reset; `query` there too, with a flag naming the brokers, so that it reads a stack from a container;
- a user's cleaning or source plugin in an image built on the platform's (its program in `/usr/local/bin`, named in its
  catalog by its name), the tutorial's advanced level; the medium level's follower in a container;
- a guard in acquisition against two runs on one stack, by any means they are started;
- `qi-v2-services`' Compose files as a product of their own (`docs/decisions.md`, Open);
- the platform's Compose file published other than in the repository at the release's tag;
- the Dockerfile's `golang` digest in `make locks-check`, which asks `flake.lock`'s inputs and the project's own locks;
  market's Go form as a Go module version, which the library taken outside this repository needs too (milestone 2);
- an image for a system other than x86_64-linux (qi-v2's Ubuntu is pinned for it alone);
- CI, building and checking the image (`docs/decisions.md`, Open, CI in the flake's shell);
- the flake package's programs as apps (`nix run`);
- a host whose user is not 1000, whose key and folder the container's user 1000 reads and writes.
