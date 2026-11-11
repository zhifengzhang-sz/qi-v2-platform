# The milestones

What the work is for. `docs/opening.md` says where it stopped; `docs/decisions.md`, who decided what and why.

## 1 The working space, built on the definition

**What.** The working space the platform's definition defines (`qi-v2-platform-definition` 0.1.0, the input
`definition`), in Go, built from the ground up, one product at a time, each built on the ones before it:
`abstract-level` (the definition's interfaces in code: the open parts of the four components and the wiring of a
working space, what the definition fixes built in), `library`, `topic-schemas`, `kafka-binding`, `acquisition`,
`warehousing`, `visualization`, `working-space`. A product that exists and meets the standard is taken as it is. The
standard: it works with the flake's `qi-v2` as pinned today (`make verify` passes); no package it uses has changed its
version since it was released; everything of the qi-v2 projects it takes is an earlier product, at the version taken
there; it satisfies the platform's definition, and is Go.

**Stands.** The definition is taken, pinned by `flake.lock`, and the shell names its package (`QI_DEFINITION`). The
library is there (`lib/`, `lib/README.md`), checked by `make check-lib`. `abstract-level`'s Go module is there
(`abstract/`, `abstract/README.md`), checked by `make check-abstract`: its tests run a working space in memory and hold
what the wiring's theorems state, and its README's table, from each declaration of the definition to its Go name, is
held to the pinned definition; it is released, the flake package `abstract-level` 0.1.0, the tag
`abstract-level-0.1.0`, checked as a taker takes it (`make check-abstract-package`). The build before this one is
cleared away: it is at the tag `old-build`, with its released products at their own tags.

**Left.** Each other product, in the order above, `library` the next. CI, none today (`docs/decisions.md`, Open).
