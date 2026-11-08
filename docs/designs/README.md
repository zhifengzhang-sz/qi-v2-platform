# The designs

A document a later session relies on goes here, tracked: a design, a review's held findings, a brief. Long output
that is made again goes to `.reports/`, ignored by git (`docs/process.md`, The files, by kind).

| Design | What |
|---|---|
| [`design-chart-consumer-2026-11-09.md`](design-chart-consumer-2026-11-09.md) | the charting consumer: one series' chart in a browser, drawn by `@qi/chart` and updated as its K-lines come; designed, not built |
| [`design-platform-image-2026-11-09.md`](design-platform-image-2026-11-09.md) | the platform as an image: the first path's programs as a flake package and one OCI image on GHCR, pinned by digest, run by Docker alone beside the test stack by a Compose file of the platform's, a user's component a container of its own built by a Dockerfile; designed, not built |
