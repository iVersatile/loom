---
name: import-enrich
description: Enrich a devcontainer import draft (loom.imported.yml) with the AI-judgment layer loom adds over a bare devcontainer (ADR-0003 — the "import & enrich, never degrade to" value-add). Use AFTER `loom import <devcontainer.json>` produces a draft, especially when the draft carries a REPORTED section (reported.unmapped_features / reported.commands) or you want a richer playbook than the deterministic Stage-1 mapping. Maps devcontainer `features` → loom `tools`, infers the `stack`, and translates the captured `reported.commands` into loom-native tools/setup. Edits ONLY the draft; never `loom.yml`; flags low-confidence inferences for the human.
---

# import-enrich — the AI layer over a devcontainer import

`loom import` (Stage-1, deterministic) maps the fields with a clean playbook home —
`ports`, `env` names, recognized `features` → `tools` — and **REPORTS** what needs
judgment into the draft's non-executable `reported:` section: unrecognized features
(`reported.unmapped_features`) and the devcontainer lifecycle commands
(`reported.commands`). It captures, it never executes — auto-running an imported
command would be a code-execution surface the guardrails must not open (ADR-0005), so
loom leaves the commands as REPORTED data for you to translate. This skill is that
judgment layer (ADR-0003): it reads the import draft + the original `devcontainer.json`
and enriches the draft, so the result is a real Loom playbook, not a thin shell.

**It is interpretation, not a parser.** The deterministic mappings already happened
in Go. Your job is the part a table can't do: infer intent, and be honest about
confidence.

## Inputs (read all three before editing)
1. The draft `loom.imported.yml` that `import` wrote (project-tier playbook).
2. The original `devcontainer.json` (the `source` from the import report).
3. The import `--json` report — its `reported` map names what was captured for you
   (`image`, `unmapped_features`, and the lifecycle hooks under `commands`); the
   command BODIES live in the draft's `reported.commands` (each with its hook + the
   command string(s)). The `deferred` list is empty in Stage-1 (everything has a home).

## What to enrich

### 1. `reported.unmapped_features` → `tools` (judgment)
`import` ALREADY mapped the **recognized official** features (`ghcr.io/devcontainers/
features/<name>`) deterministically into `tools:` — you do not redo those. Your job is
the leftovers: the refs `import` surfaced in **`reported.unmapped_features`** (custom,
third-party, or official-but-unlisted), which it deliberately did NOT guess.
For each unmapped ref:
- If you can confidently identify the tool it installs (its name/docs make it obvious),
  add a `tools:` entry (`name@version` intent; the resolver/lockfile pins the build) and
  say why. Take a version from the feature's `version` OPTION when present, else bare name.
- Officially-named but loom-inappropriate features (e.g. `docker-in-docker` — loom is
  container-per-project, not DinD; `common-utils` — a meta feature) → a `# REVIEW:` note,
  not a tool.
- **Anything you are not confident about → a `# REVIEW:` comment** naming the ref and
  what it appears to install. Never guess a tool name (the deterministic mapper already
  declined to — don't override its caution).

### 2. infer the `stack`
Loom's `stack:` selects a `stacks/<lang>` overlay. Infer it from the strongest signal,
in order: an explicit language feature (a `go`/`node`/`python`/… feature → that stack),
then the devcontainer `image` (e.g. `mcr.microsoft.com/devcontainers/go` → `go`), then
the project files if visible. Set `stack:` only when you are confident; otherwise add a
`# REVIEW: stack unclear (signals: …)` note rather than a wrong overlay.

### 3. `reported.commands` → translate, do NOT auto-map or run
`import` CAPTURED the devcontainer lifecycle commands (`postCreateCommand`,
`onCreateCommand`, …) verbatim into the draft's **`reported.commands`** — each entry a
`hook` + the command `run` string(s). They are REPORTED data, **never executed**: loom
has no inline-command field, it provisions declaratively (tools) and uses hooks
(guardrail references, not arbitrary shell), and auto-running an imported command would
be a code-execution surface the guardrails must not open (ADR-0005). Your job is to
TRANSLATE them, not run them:
- Do NOT fabricate a tool or hook to "run" a command, and never move a command into an
  executable field (there is none; that is deliberate).
- For each `reported.commands` entry, decide its loom-native shape and record it as a
  `# REVIEW:` note (the human applies it), e.g.:
  `# REVIEW: postCreateCommand "npm install" — a dependency install; loom installs`
  `#   declared tools at provision. If a tool is missing, add it to tools:.`
- If a command clearly just installs a tool already covered by a feature/stack, say so;
  never invent build steps.
- Once a command is translated (or judged a no-op), you MAY remove that entry from
  `reported.commands` — the section is review scaffolding, not desired-state, and the
  engine ignores it either way. Anything you cannot confidently translate, LEAVE in
  `reported.commands` (lossless) and flag it. Never silently drop an untranslated command.

### 4. the `image`
`import` REPORTED the devcontainer image; it is intentionally NOT in the playbook —
loom enriches with its own curated base (ADR-0012/0003), it does not adopt the
devcontainer's image. Leave a one-line note that the original image was `<X>` and that
`LOOM_BASE_IMAGE` is the override path if the user truly needs that exact base. Do not
add an `image:`/`base_image:` field (there is none).

## Discipline (non-negotiable)
- **Edit only the draft** (`loom.imported.yml`). Never touch `loom.yml`, the source
  `devcontainer.json`, or anything else.
- **Names-only env**: never write credential VALUES into the playbook; `env:` carries
  names only (values live in `.env`/secret store). Do not invent secrets.
- **Flag, don't guess**: every low-confidence inference is a `# REVIEW:` comment, not a
  silent decision. A wrong overlay or a hallucinated tool is worse than a deferral.
- **Keep it valid**: the draft must still parse + validate as a project-tier playbook
  (run `loom plan -f loom.imported.yml` or re-read it) after your edits.
- This is a **draft for the human to review then commit** — preserve that posture; do
  not present it as final.

## Output
The enriched `loom.imported.yml` plus a short summary: what you mapped (features→tools,
stack), what you left as `# REVIEW:` notes (commands, low-confidence features), and the
reported image. Lead with the confidence level so the human knows what to check.
