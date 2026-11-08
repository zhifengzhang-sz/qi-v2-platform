# The platform's agents: its spec

This project's spec of the work agents do on it, in no tool's terms: `rules.md` (the standing rules), `roles/` (the
roles), `stages/` (the processes' stages; none yet). The format, and the kit that writes each kind's files from it
(Claude Code, Copilot), are the kit's: `dev/agents/README.md` once installed (`make install`), its source the flake
`kit/` of the public repository `qi-v2-nix`. After a change here: `make agents`. The generated files (`AGENTS.md`,
`CLAUDE.md`, `.claude/agents/`, `.github/agents/`) are built output, ignored by git, never edited.

## Its roles

| Role | For | Writes | Model |
|---|---|---|---|
| [checker](roles/checker.md) | runs the make targets named and reports their lines | none | small |
| [reviewer](roles/reviewer.md) | tries to refute another's work | none | strongest; other when it is the other model |
| [writer](roles/writer.md) | drafts documentation from the project's files | brief-files | middle |
| [editor](roles/editor.md) | makes a decided, mechanical change and shows the checks pass | the-change | middle |
| [researcher](roles/researcher.md) | researches a topic on the web into a notes file | brief-files | middle |
