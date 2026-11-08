# Compose application as an OCI artifact: `docker compose publish` and `-f oci://` (notes, 2026-11-09)

Reading marks: [READ] = read in the source named; [SUMMARY] = only a fetch tool's or search engine's summary of a page, not the page itself; [INFER] = my inference from what was read.
Only public HTTPS pages and public repositories; no `gh`, no token, no registry login; only `docker compose ... --help` and `docker compose version` run.
Code was read at the tag `v5.7.0` of github.com/docker/compose (the version on this machine), as raw files from raw.githubusercontent.com. Other tags are named where used. compose-go read at `v2.16.2` (the version `v5.7.0`'s go.mod names).
Docker Engine 29.9.3 / Compose v5.7.0 (`docker compose version` -> v5.7.0, [READ] locally).

## Sources

- S1 publish code: https://raw.githubusercontent.com/docker/compose/v5.7.0/pkg/compose/publish.go
- S2 publish command: https://raw.githubusercontent.com/docker/compose/v5.7.0/cmd/compose/publish.go
- S3 artifact push, media types: https://raw.githubusercontent.com/docker/compose/v5.7.0/internal/oci/push.go and .../internal/oci/resolver.go
- S4 remote OCI loader: https://raw.githubusercontent.com/docker/compose/v5.7.0/pkg/remote/oci.go (env default also read at tags v2.30.0, v2.32.0, v2.33.0, v2.34.0, v2.36.0)
- S5 run-time prompts: https://raw.githubusercontent.com/docker/compose/v5.7.0/cmd/compose/options.go and .../cmd/compose/up.go
- S6 project name and working dir: https://raw.githubusercontent.com/compose-spec/compose-go/v2.16.2/cli/options.go ; .../loader/include.go ; .../loader/loader.go
- S7 secrets and configs injection: https://raw.githubusercontent.com/docker/compose/v5.7.0/pkg/compose/secrets.go
- S8 docs: https://docs.docker.com/compose/how-tos/oci-artifact/ ; https://docs.docker.com/reference/cli/docker/compose/publish/ (both [SUMMARY] via fetch tool)
- S9 release v2.34.0: https://github.com/docker/compose/releases/tag/v2.34.0 ([SUMMARY] via fetch tool)
- S10 compose-spec: https://raw.githubusercontent.com/compose-spec/compose-spec/main/08-configs.md ; .../09-secrets.md ; .../schema/compose-spec.json
- S11 GitHub docs source: https://raw.githubusercontent.com/github/docs/main/content/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility.md ; .../content/packages/working-with-a-github-packages-registry/working-with-the-container-registry.md
- S12 Docker Hub docs source: https://raw.githubusercontent.com/docker/docs/main/content/manuals/docker-hub/usage/pulls.md (rendered: https://docs.docker.com/docker-hub/usage/pulls/)

## 1. publish: what it publishes, flags, version, env vars

1.1 `publish` pushes an OCI image manifest (artifactType `application/vnd.docker.compose.project`) whose layers are the Compose files, media type `application/vnd.docker.compose.file+yaml`, plus (with `--with-env`) env-file layers, media type `application/vnd.docker.compose.envfile`; config is the OCI 1.1 empty descriptor (`application/vnd.oci.empty.v1+json`, `{}`).
- Source: S3 push.go consts `ComposeProjectArtifactType`, `ComposeYAMLMediaType`, `ComposeEnvFileMediaType`; `generateManifest` (case OCIVersion1_1: `config = v1.DescriptorEmptyJSON; artifactType = ComposeProjectArtifactType`). Manifest annotation `org.opencontainers.image.created`; descriptor annotation `com.docker.compose.version`. [READ]

1.2 If the registry rejects the OCI 1.1 form with a non-auth 4xx, it falls back to OCI 1.0: no artifactType, config media type `application/vnd.docker.compose.config.empty.v1+json`; publish then prints a warning "published (registry rejected OCI 1.1; fell back to OCI 1.0)". `--oci-version` forces one.
- Source: S3 `PushManifest`, `isNonAuthClientError`; S1 `publish` (text/status Warning). [READ]

1.3 One layer per Compose file passed with `-f` (each as published text, not resolved), one extra layer per local `extends` parent file (annotation `com.docker.compose.extends`, name = hash), and with `--resolve-image-digests` one more layer named `image-digests.yaml`, an override file holding only `services.<n>.image: <name@sha256:...>` (and the same for jobs).
- Source: S1 `createLayers`, `processExtends`, `generateImageDigestsOverride`. [READ]
- The raw published text is the file as written (interpolation kept symbolic); only `env_file` and local `extends.file` entries are rewritten to hash names. S1 `processFile` (uses `transform.ReplaceEnvFile`, `transform.ReplaceExtendsFile` on the original bytes `f`). [READ]

1.4 `--resolve-image-digests`: pins each service image tag to its digest by asking the registry, and ships the result as the extra `image-digests.yaml` layer (not by editing your file). Needs network and registry access to each image at publish time.
- Source: S2 flag text "Pin image tags to digests"; S1 `generateImageDigestsOverride` (`project.WithImagesResolved(imageDigestResolver(...))`). [READ]
- At run time the loader writes all Compose-file layers into one `compose.yaml`, separated by `\n---\n`, so the override is a second document that merges over the first (multi-document = several files merged). Source: S4 `writeComposeFile`. [READ for the writing; the merge semantics of a `---` multi-document file are INFER]

1.5 `--app`: sets `ResolveImageDigests = resolveImageDigests || app` (so `--app` implies digest pinning), and after pushing the artifact copies every service image (and job image) into the same repository as the artifact and pushes an OCI image index (annotation `com.docker.compose.version`) whose manifests are the copied images, with `subject` = the Compose artifact manifest; the index is pushed by digest in the artifact's repository (`TrimNamed(named)`, no tag). The result: the application and its images are in one repository.
- Source: S2 `runPublish` (`ResolveImageDigests: opts.resolveImageDigests || opts.app`); S1 `pushComposeArtifact`, `pushApplicationIndex`; `oci.Copy` in S3 resolver.go (copies the chain from the source image to the artifact repo with a registry mount hint). [READ]
- `--app` first appears in v2.40.0 (flag present at v2.40.0, absent at v2.39.0). Source: cmd/compose/publish.go at tags v2.34.0 to v2.40.0, grep for `"app"`. [READ]
- What the index digest vs the manifest digest is as the pin to give to `-f oci://...@sha256:` is not shown by publish.go (the digest printout was not looked for). At run time the loader accepts either: if the descriptor is an index it picks the manifest(s) with artifactType `application/vnd.docker.compose.project`. S4 `resolveComposeManifest`. [READ]
- A subtlety: the `--app` flow copies images to the artifact repo; for a platform image already on GHCR it is a copy within the registry, for Docker Hub-hosted images (Redpanda, ClickHouse, TimescaleDB) it is a copy from Docker Hub to GHCR (an anonymous or logged-in pull from Hub is needed by whoever publishes). [INFER from `oci.Copy`]

1.6 `--with-env`: includes in the artifact the `env_file` files the services declare (as layers `application/vnd.docker.compose.envfile`, named `<sha256 of path>.env`). It does not include the shell environment, nor `.env` as such. It also silences the interactive prompt about `env_file` and sensitive-looking literal environment values.
- Source: S1 `createLayers` (`if options.WithEnvironment { layers = append(layers, envFileLayers(envFiles)...) }`), `envFileLayers`, `checkEnvironmentVariables` (`if !options.WithEnvironment && findings.hasEnvFinding()` prompt). [READ]
- Without `--with-env`, `env_file` entries are still rewritten to `<hash>.env` in the published YAML but the files are not uploaded (comment in S1: "the env_file is always rewritten to its opaque <hash>.env placeholder"); so a required `env_file` would be missing at run time. [READ for the rewriting; the failure at run is INFER]

1.7 Generally available (not experimental) from Compose v2.34.0: release note "Make `publish` a regular command of Compose" (#12629). Before that the command was `docker compose alpha publish`.
- Source: S9. [SUMMARY] of the release page. The docs page for the how-to also names no minimum (S8, [SUMMARY]); a search summary said the how-to page says "requires Docker Compose 2.34.0 and later" (not read on the page).
- The remote loader is on by default from v2.34.0: `ociRemoteLoaderEnabled` returns `true` when `COMPOSE_EXPERIMENTAL_OCI_REMOTE` is unset; at v2.30.0, v2.32.0, v2.33.0 it returned `false`. [READ, raw oci.go at those tags]

1.8 No environment variable is needed to publish or to run `-f oci://` on v5.7.0. `COMPOSE_EXPERIMENTAL_OCI_REMOTE` still exists but only to switch it off (`false`); unset means enabled. No `COMPOSE_EXPERIMENTAL`-style gate on `publish` was found in S1/S2.
- Source: S4 `ociRemoteLoaderEnabled`, `Load` ("OCI remote resource is disabled by ..."). [READ]

## 2. What publish refuses or warns about (v5.7.0 code)

2.1 Bind mounts: asks for confirmation (default no) and, if confirmed, publishes the declaration only (no content). With `-y` it goes through. At v2.34.0 to v2.37.0 it was a hard refusal ("cannot publish compose file: service %q relies on bind-mount. You should use volumes"); the prompt form is in v2.38.0 onward.
- Source: S1 `preChecks`, `checkForBindMount` (v5.7.0); v2.34.0 p_ file `checkForBindMount` error text; tag scan for the prompt text: absent at v2.37.0, present at v2.38.0. The docs page (S8, [SUMMARY]) still lists "services with bind mounts" among what you can't publish: out of date against the code. [READ code, SUMMARY docs]
- Covers `type: bind` in service `volumes` (short and long syntax) of services and jobs. It does not look at configs/secrets.

2.2 `configs` / `secrets` whose source is a local `file`: not refused. The file's content is scanned for secrets (Defang secret-detector) and, if something is detected, a confirmation prompt appears; the file's content is NOT uploaded. The published YAML keeps the `file:` path as written, so the application would not run for someone without that file.
- Source: S1 `checkForSensitiveData` ("Check configs defined by files", "Check secrets defined by files"; `scanFiles`), and `processFile` publishing the original bytes. [READ]; "would not run for someone else" is INFER.

2.3 `configs: x: content:` (inline): allowed. If the content looks literal (not just `${VAR}` interpolation) publish always asks for confirmation ("you are about to publish literal inline config content") - `-y` answers yes. `environment:` configs (a variable name) never prompt.
- Source: S1 `collectEnvCheckFindings`, `configContentLooksLiteral`, `checkEnvironmentVariables`, `buildConfigContentPromptMessage`. [READ]

2.4 `secrets` with `environment:`: allowed, only the variable name is published; no prompt for it (secrets are not in the env scan, only configs are). The literal-environment scan covers the services' `environment:` values with sensitive-looking keys (password, secret, token, api_key ...): a prompt, silenced by `--with-env`. `${VAR}` interpolated values are kept symbolic and do not prompt.
- Source: S1 `recordEnvFindings`, comment "Interpolated values like "${SECRET}" or "$VAR" are preserved as placeholders". [READ]

2.5 `build` sections: a service (or job) with `build` and no `image` -> hard error "your Compose stack cannot be published as it only contains a build section". A service with both `image` and `build` passes. Also, `publish` first runs a push of built images (`s.push` with `ImageMandatory: true`, `IgnoreFailures: true`).
- Source: S1 `checkOnlyBuildSection`, `publish` (call to `s.push`). [READ]

2.6 `env_file`: allowed; prompt unless `--with-env` (see 1.6); a missing required `env_file` fails earlier in project loading; a missing optional one is rewritten but not uploaded.
- Source: S1 `processFile`, `scanEnvFiles`, `checkEnvironmentVariables`. [READ]

2.7 `include` of another local file: hard error "cannot publish compose file with local includes" (counted from the loader's `include` event for paths no remote loader accepts). `include` of a remote `oci://` or Git application: allowed (counted as a remote include); the include stays symbolic in the published file. v2.34.0 also added a warning for a remote configuration that includes another remote one ("Add warning message when a remote configuration include an another remote config", #12610, [SUMMARY] S9).
- Source: S2 `runPublish` (`metrics.CountIncludesLocal > 0`); S5/options.go `ToProject` metrics listener; S1 `processFile` (`options.SkipInclude = true`, comment in `loadUnresolvedFile`: "include directives stay symbolic in the published artifact"). [READ]
- The included remote artifact is NOT copied into your artifact and is not pinned by publish (`--resolve-image-digests` pins images only). So pin the include in your file with `@sha256:` yourself. [INFER; read: nothing in S1 rewrites include paths]

2.8 `extends`: a local `extends.file` is published as an extra layer (hash name, annotation `com.docker.compose.extends`), recursively; a remote `extends` is left as is; an `extends` within the same file needs nothing.
- Source: S1 `processFile`, `processExtends`, `localExtendsParent`. [READ]

2.9 Other pre-checks: the whole Compose files, resolved (without interpolation), are scanned with the secret-detector (AWS keys, GitHub tokens, JWTs, private keys...) -> prompt if found. `-y` / `--yes` answers all prompts yes. Prompts need a TTY or `-y`.
- Source: S1 `checkForSensitiveData`, `composeFileAsByteReader`; S2 `--yes`; S8 [SUMMARY]. [READ code]

## 3. Running one

3.1 Forms: `docker compose -f oci://<registry>/<repo>[:tag][@sha256:...] up -d`. The path after `oci://` is parsed by `reference.ParseDockerRef` (distribution/reference), which takes `name`, `name:tag`, `name@digest`, `name:tag@digest`. The digest form is therefore accepted by the parser and resolved with `resolver.Resolve`; blob digests are re-verified (`GetBlob`). `include:` takes the same `oci://` string (the `Accept` test is the `oci://` prefix and is the same loader for `-f` and `include`).
- Source: S4 `pullComposeArtifact`, `Accept`; S6 loader/include.go (`loader.Accept(p)` -> `loader.Load`); `oci.GetBlob` in S3 resolver.go. [READ]. That a pull by `@sha256:` works end to end against GHCR was not run (no pull allowed) - [INFER].
- Docs example: `docker compose -f oci://docker.io/username/my-compose-app:latest up` (S8 [SUMMARY]).

3.2 Cache and relative paths: the artifact is extracted to `<cache>/<digest hex of the resolved descriptor>/compose.yaml` (cache = `$XDG_CACHE_HOME/docker-compose` else the OS cache dir), a `.complete` marker written last. Unless `--project-directory` is given, the project directory is that folder, so a relative path in the application (`./config/x`) points into that cache folder, where only `compose.yaml`, extends files and env files exist.
- Source: S4 `pullComposeArtifact`, `cacheDir` (pkg/remote/cache.go); S6 cli/options.go `ReadConfigFiles` (comment: "a remote resource loader (git, oci) does not override it with the directory of its own downloaded copy" when the working dir is explicit), loader.go lines ~364 (`config.WorkingDir = filepath.Dir(local)` if not explicit). [READ]
- For an `include:` of an `oci://` app, the included app's `project_directory` defaults to the folder of its extracted `compose.yaml`; relative paths resolve there (S6 include.go: `r.ProjectDirectory = filepath.Dir(path)`). [READ]

3.3 Project name: precedence is `-p`, then `COMPOSE_PROJECT_NAME`, then top-level `name:` in the file, then the basename of the working dir. For `-f oci://...` without `--project-directory`, the working dir is the cache folder, so the fallback name would be the digest hex. So: put a top-level `name:` in the file (or pass `-p`).
- Source: S6 cli/options.go `withNamePrecedenceLoad`, `ProjectOptions.Name` handling; S4 (cache folder = digest hex). [READ]; the digest-hex name as the result is [INFER] from those two.

3.4 Prompts at `up`: for a remote main file, unless `-y`/`--yes` is given, `up` prints a table of the interpolation variables with their VALUES (from the command line or environment), whether required, and default, and asks "Do you want to proceed with these variables?" (default yes); with remote includes it also warns "Remote includes could potentially be malicious" and asks to continue (default no). It prints the cache location. A user's API key that is read by `${VAR}` interpolation would be echoed to the terminal in that table when no `-y` is given. `-y` skips both prompts ("Assume "yes" as answer to all prompts and run non-interactively").
- Source: S5 options.go `checksForRemoteStack`, `promptForInterpolatedVariables`, `extractInterpolationVariablesFromModel` (`info.value = value` for `os.LookupEnv`), `confirmRemoteIncludes`; up.go `--yes` flag and call at line ~235. [READ]
- A key given only as `secrets: x: environment: VAR` is not interpolation, so it is not in that table. [INFER: the extractor reads `${...}` patterns via `template.ExtractVariables`]

3.5 Remote app bind-mounting a host path chosen by the user: nothing in the loader forbids it. Absolute paths and `${HOME}/...` (interpolated from the user's environment) work in plain Compose like in a local file; only the declaration is in the artifact. No restriction on bind mounts was found in S4/S6; the only check is at publish (2.1).
- Source: absence in S4, S5, S6 [READ]; behaviour at run [INFER]. Relative bind sources resolve in the cache folder (3.2); not run.

3.6 `secrets: x: environment: VAR` in plain (non-Swarm) Compose: supported (Compose v2.6.0 per the spec badge). Compose reads `project.Environment[VAR]` (the process environment plus the `.env` of the project dir) and writes it into the container by `CopyToContainer` as a file `/run/secrets/<name>` (mode 0444 by default; target/uid/gid/mode supported), after the container is created and before it starts; error "environment variable ... is not set" if absent; refused for a `read_only: true` service. The secret never touches the host disk and is not in the artifact.
- Source: S10 09-secrets.md ("`environment`: The secret is created with the value of an environment variable." badge v2.6.0); S7 `injectFileReferences`, `resolveFileContent`, `copyFileToContainer`. [READ]
- For a remote main file the `.env` file is not read (`setEnvWithDotEnv` returns early for a remote path), so the variable must be in the process environment. Source: S5 compose.go `setEnvWithDotEnv` (compose.go lines ~754-765). [READ]

## 4. Inline content

4.1 `configs: x: content: |` (inline text) is in the Compose specification, from Compose v2.23.1 (badge in the spec). In plain Compose it is injected like 3.6: written into the container with `CopyToContainer` (default target `/<name>`, mode 0444), interpolation applies to the content (spec example uses `${DEBUG}`).
- Source: S10 08-configs.md (`content`: "The content is created with the inlined value." + badge v2.23.1; Example 2); S7 `resolveFileContent` (`if source.Content != ""`). [READ]
- Compose v5.7.0 is well past 2.23.1.

4.2 `secrets: x: content:` does not exist: the spec's secret has `file`, `environment`, `external` only; the schema properties for `secret` are name, environment, file, external, labels, driver, driver_opts, template_driver (no `content`); for `config`: name, content, environment, file, external, labels, template_driver. compose-go validation: "secret %q must declare either `file` or `environment`".
- Source: S10 schema/compose-spec.json (`$defs.secret.properties`, `$defs.config.properties`, read by script); compose-go v2.16.2 loader/validate.go. [READ]
- Note: compose-go's `FileObjectConfig` Go type has a `Content` field shared by both types, but the schema rejects it for secrets. [READ types.go]; the consequence "rejected" is INFER from the schema (schema validation is on by default).
- Inline config content is published as literal text, with the prompt of 2.3 (only for `-y` absent). The Console config (YAML) can therefore be a `configs: content:`, mounted by `configs: [{source:, target: /etc/redpanda/redpanda-console-config.yaml}]`. [INFER, not run]

## 5. GHCR

5.1 GHCR docs say the registry supports "Docker Image Manifest V2, Schema 2" and "Open Container Initiative (OCI) Specifications" (and foreign layers) and "You can store and manage Docker and OCI images". The page does not mention OCI artifacts, artifactType or custom media types. That GHCR stores and serves a Compose artifact of the media types in 1.1 is NOT established by GitHub documentation.
- Source: S11 working-with-the-container-registry.md (intro line, formats list). [READ]
- Related facts that bear on it: Compose pushes with a fallback for registries that reject the OCI 1.1 form (artifactType and `application/vnd.oci.empty.v1+json`) and then uses the OCI 1.0 form with a custom config media type (1.2). So a registry that takes any custom config media type works in one of the two forms. That GHCR does is not read anywhere; a search found no ghcr.io-specific Compose-artifact guide. [INFER; GAP]

5.2 A public package is pulled with no login: "public packages allow anonymous access and can be pulled without authentication" (the Container registry). Pushing, and pulling private ones, needs a PAT (classic) with `write:packages` / `read:packages`.
- Source: S11 ghcr docs ("You can also access public container images anonymously" - via the fetch tool [SUMMARY]); the configuring-visibility source text was read in raw for visibility and inheritance, the "anonymous" sentence read via the fetch tool [SUMMARY].
- No published limit on anonymous pulls of ghcr.io was found in GitHub's docs (grep of both docs files for "rate" gave nothing relevant; the only limits in the docs: 10 GB per layer, 10-minute upload timeout [SUMMARY]). Third-party pages disagree with each other (200 per 6 hours; 5,000 per day; no limit); none is a GitHub source. GAP: the anonymous limit of GHCR is not established.

5.3 Default visibility: for a package first published into a personal account, private ("When you first publish a package that is scoped to your personal account, the default visibility is private and only you can see the package"); same for an organization. Making it public: package settings -> Danger Zone -> Change visibility; "Once you make a package public, you cannot make it private again."
- Source: S11 configuring-a-packages-access-control-and-visibility.md lines 179, 187, 208, 216. [READ]
- Public by `org.opencontainers.image.source`: NO. That label (or a connect in settings) links the package to a repository; the package then inherits the repository's access permissions "(but not the visibility)". Visibility stays private until it is changed by hand. Inheritance also only happens automatically if the repository is linked before the first publish (label); connecting afterwards leaves permissions as they are unless chosen.
- Source: same file line 26, 28 (quoted); ghcr.md line 196 (the label). [READ]
- Practical point for an artifact: the source label is a Dockerfile label or an image annotation; `compose publish` sets only `org.opencontainers.image.created` on the manifest and offers no flag to add annotations (S3 `generateManifest`, S2 flags). So for the artifact the link to the repository would be made in the package settings (or the first push from a workflow of the repository). [INFER; the GitHub docs line 223 mention that annotations in a multi-arch manifest are used for the description]

## 6. Docker Hub anonymous pulls

6.1 Unauthenticated pulls: 100 per 6 hours, per IPv4 address or per IPv6 /64 subnet; Personal (authenticated) 200 per 6 hours; Pro, Team, Business unlimited (fair use). A normal image pull counts one manifest pull; multi-arch counts one per architecture.
- Source: S12 pulls.md table and "Pull definition". [READ]

6.2 Since when: the page (S12) carries no date. A February 2025 plan to cut unauthenticated pulls to 10 per hour (effective 1 March, then 1 April 2025) was not enforced; Docker's blog update of 8 April 2025 says 100 per 6 hours (unauthenticated) and 200 per 6 hours (Personal) stay. The 100/6h figure is the one introduced with the rate limits in November 2020 (my memory and one search summary: "these rate limits were added to Docker Hub in 2020") - NOT read on a source page in this session.
- Source: search summary of https://www.docker.com/blog/revisiting-docker-hub-policies-prioritizing-developer-experience/ [SUMMARY, page not read]; https://www.docker.com/blog/scaling-docker-to-serve-millions-more-developers-network-egress/ read: gives dates (Dec 10 2024, Mar 1 2025) but no numbers. GAP: the effective date of the current 100/6h limit is not established from a read primary source.

## Gaps (not established)

- G1 Whether GHCR accepts and serves a Compose artifact (either 1.1 or 1.0 form) and an image index with `subject` - not documented by GitHub; needs a real push, which was not allowed.
- G2 Any published limit on anonymous pulls of ghcr.io.
- G3 Date from which Docker Hub's anonymous limit is 100 per 6 hours (primary source not read).
- G4 What digest `publish` prints, or which digest (manifest vs index with `--app`) to pin; whether `publish` output shows it. Not looked for in the code; the loader accepts both (1.5).
- G5 Behaviour of `-f oci://...@sha256:...` end to end (parser and resolver accept it by code reading; not run).
- G6 The docs how-to (S8) was read through a fetch tool summary; its text about bind mounts disagrees with the code from v2.38.0 (prompt, not refusal).
- G7 The release page for v2.40.0 (where `--app` appears) was not read; the version was found by the presence of the flag in the source at tags.
- G8 Behaviour of multi-document `compose.yaml` (`---`) merging for `image-digests.yaml` was inferred, not run.
