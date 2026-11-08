# The designs

A document a later session relies on goes here, tracked: a design, a review's held findings, a brief. Long output
that is made again goes to `.reports/`, ignored by git (`docs/process.md`, The files, by kind).

| Design | What |
|---|---|
| [`design-chart-consumer-2026-11-09.md`](design-chart-consumer-2026-11-09.md) | the charting consumer: one series' chart in a browser, drawn by `@qi/chart` and updated as its K-lines come; designed, not built |
| [`design-platform-image-2026-11-09.md`](design-platform-image-2026-11-09.md) | the platform as an app: one Compose app on GHCR running the first path's programs from one image with the Redpanda and TimescaleDB they need, by Docker alone and a Massive key; the programs as a flake package for another project; a user's component a container of its own built by a Dockerfile; designed, not built |
| [`notes-compose-publish-2026-11-09.md`](notes-compose-publish-2026-11-09.md) | a researcher's sourced notes on Compose's published apps (`docker compose publish`, `-f oci://`), GHCR's visibility and Docker Hub's pull limits, which the design above rests on |
