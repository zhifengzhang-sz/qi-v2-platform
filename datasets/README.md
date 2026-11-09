# The K-line data sets

Each a product with a version: a fixed set of K-lines of one of the platform's clean series over a range of its
sessions, made from the record, published as an image on GHCR, one layer a file, and pinned by its lock here, which
names the image's tag and digest and every file's sha256. The files are in the image alone, never in git. A version
once released never changes, so anything made from a data set names the version it was made from. A Nix taker takes
a version by the platform's flake, the product `datasets`: its package installs the version's files into the Nix store
from GHCR with no login, and its function `lib.dataset` gives a package each file by its hash. What a data set holds,
its form, its versions, how it is made and how a taker pins one, reads it and takes it by Nix:
[the design](../docs/designs/design-klines-dataset-2026-11-09.md).

| Data set | Version | Its series | Its range | K-lines | Files | Image |
|---|---|---|---|---|---|---|
| `qi-v2-platform-klines-massive-spy-1m` | 1.0.0 | `market.clean.validation.v0-1-0.massive.SPY.M1`, the validation 0.1.0's clean series of SPY's minutes from Massive | the sessions 2024-11-06 to 2026-11-06 in `America/New_York`, 502, every one whole (497 of 390 minutes, 5 early closes of 210), from 09:30 to 15:59 | 194,880 | 25, `2024-11.jsonl` to `2026-11.jsonl`, 21,837,727 bytes | `ghcr.io/zhifengzhang-sz/qi-v2-platform-klines-massive-spy-1m:1.0.0`, public, its digest in [its lock](qi-v2-platform-klines-massive-spy-1m.lock), its layers 3.9 MB |

**Reading one**, with no login, the digest the lock's at the version's tag (the design, How a taker pins one and reads
it):

    crane export ghcr.io/zhifengzhang-sz/qi-v2-platform-klines-massive-spy-1m@<digest> - | tar -x -C <folder>
    jq -r '.files[] | "\(.sha256)  \(.path)"' qi-v2-platform-klines-massive-spy-1m.lock | (cd <folder> && sha256sum --check --strict)

**Taking one by Nix** (the design, How a Nix taker takes a version): the taker's flake pins the platform at a tag whose
lock is the version, `datasets-1.0.0` for SPY's 1.0.0, and its package takes the files by `lib.dataset`:

    inputs.platform.url = "github:zhifengzhang-sz/qi-v2-platform?ref=datasets-1.0.0";
    spy = platform.lib.dataset { inherit pkgs; dataset = "qi-v2-platform-klines-massive-spy-1m"; version = "1.0.0"; };
    # spy.files."2024-11.jsonl", each file by its hash; spy.folder, every file in one folder

then, in its folder, its lock written, the version installed from GHCR with no login, a file the store holds not
fetched again, and its package built:

    nix flake lock
    nix run --inputs-from . platform#datasets -- qi-v2-platform-klines-massive-spy-1m 1.0.0
    nix build

**Making one** (the design, How it is made): `make dataset`, from the stack's clean topic, read only, twice to the
same bytes, its lock written here; `make dataset-check`, its image read back from a registry of crane's own; `make
check-datasets`, in `make check`, every lock held to its form and a released one to its tag's. Publishing it is the
owner's: `make dataset-push`, then the package set public on GHCR; then `make dataset-remote-check`, with no login, and
`make dataset-take-check`, the version taken as a Nix taker takes it.

## Changes

### `qi-v2-platform-klines-massive-spy-1m`

- **1.0.0** (tag `qi-v2-platform-klines-massive-spy-1m-1.0.0`). The first: SPY's clean minutes of the validation 0.1.0,
  the sessions 2024-11-06 to 2026-11-06, the clean topic whole as the record held it (194,880 K-lines, none refused,
  the corrected raw of Massive's 417 revisions), a file a month, each line market 1.1.0's K-line, its index its place
  from 0. On GHCR, public, pushed by the owner; then `make dataset-remote-check`, the owner's, whole:

      dataset-remote-check: ghcr.io/zhifengzhang-sz/qi-v2-platform-klines-massive-spy-1m:1.0.0: sha256:5741bcb828b8ef8482c72b2edf4c3116bb89bbcf135f86c4c80eca0e8d1e121e, the lock's, read with no login; every file (25) the lock's sha256 and size and no other

  Taken by Nix from `datasets-1.0.0` (below).

### `datasets`

- **1.0.0** (tag `datasets-1.0.0`). The first: the platform's flake's package `datasets`, its program
  `datasets-install DATASET VERSION`, and its function `lib.dataset { pkgs, dataset, version }`; the data sets of the
  commit a taker pins, SPY's 1.0.0 at this tag. `make dataset-take-check`: none of SPY's 1.0.0's 25 files in the store
  before; all 25 installed by the taker's own line from GHCR with no login, each the lock's sha256; a taker's package
  built from all 25 by their hashes; installed again, nothing fetched.
