---
name: import-enrich
description: Enrich a devcontainer import draft (loom.imported.yml) with the AI-judgment layer loom adds over a bare devcontainer (ADR-0003 — the "import & enrich, never degrade to" value-add). Use AFTER `loom import <devcontainer.json>` produces a draft, especially when the import `--json` report lists DEFERRED fields (features / commands) or you want a richer playbook than the deterministic Stage-1 mapping. Maps devcontainer `features` → loom `tools`, infers the `stack`, and surfaces `commands` as reviewable notes. Edits ONLY the draft; never `loom.yml`; flags low-confidence inferences for the human.
---

# import-enrich — the AI layer over a devcontainer import

`loom import` (Stage-1, deterministic) maps only the fields with a clean playbook
home — `ports` and `env` names — and **defers** what needs judgment: `features`,
`commands`, and the `stack`. This skill is that judgment layer (ADR-0003): it reads
the import draft + the original `devcontainer.json` and enriches the draft, so the
result is a real Loom playbook, not a thin shell.

**It is interpretation, not a parser.** The deterministic mappings already happened
in Go. Your job is the part a table can't do: infer intent, and be honest about
confidence.

## Inputs (read all three before editing)
1. The draft `loom.imported.yml` that `import` wrote (project-tier playbook).
2. The original `devcontainer.json` (the `source` from the import report).
3. The import `--json` report — its `deferred` list tells you exactly what was left
   for you (`features`, `commands`), and `reported` carries the devcontainer `image`.

## What to enrich

### 1. `features` → `tools`
Devcontainer features are OCI refs like `ghcr.io/devcontainers/features/go:1` or
`ghcr.io/devcontainers/features/node:20`. Map each to a loom `tools:` entry
(`name@version` intent; the resolver/lockfile pins the exact build):
- The tool **name** is the last path segment before the `:` (`.../features/<name>:<ver>`).
- The **version** is the tag (`:1`, `:20`, `:latest`); use `name@<tag>` when the tag
  is a clean version, else the bare name (the same rule `import` uses for tool versions).
- Common official features (high confidence):
  `go`→`go`, `node`→`node`, `python`→`python`, `rust`→`rust`, `ruby`→`ruby`,
  `java`→`java`, `dotnet`→`dotnet`, `php`→`php`, `docker-in-docker`→(skip; loom is
  container-per-project, not DinD), `github-cli`→`gh`, `aws-cli`→`awscli`,
  `terraform`→`terraform`, `kubectl-helm-minikube`→`kubectl`+`helm`.
- A **custom / unknown / non-`ghcr.io/devcontainers/features` ref** is low confidence:
  do NOT guess a tool name — leave it in a `# REVIEW:` comment naming the ref and what
  it appears to install, for the human to decide.

### 2. infer the `stack`
Loom's `stack:` selects a `stacks/<lang>` overlay. Infer it from the strongest signal,
in order: an explicit language feature (a `go`/`node`/`python`/… feature → that stack),
then the devcontainer `image` (e.g. `mcr.microsoft.com/devcontainers/go` → `go`), then
the project files if visible. Set `stack:` only when you are confident; otherwise add a
`# REVIEW: stack unclear (signals: …)` note rather than a wrong overlay.

### 3. `commands` → reviewable notes (do NOT auto-map)
`postCreateCommand` / `postStartCommand` / `onCreateCommand` etc. are arbitrary shell.
Loom has **no inline-command field** — it provisions declaratively (tools) and uses
hooks (guardrail references, not arbitrary shell). So:
- Do NOT fabricate a tool or hook to "run" a command.
- Capture each command verbatim in a `# REVIEW:` comment block at the top of the draft,
  with a one-line read of intent and the loom-native alternative, e.g.:
  `# REVIEW: postCreateCommand "npm install" — a dependency install; loom installs`
  `#   declared tools at provision. If a tool is missing, add it to tools:.`
- If a command clearly just installs a tool already covered by a feature/stack, say so
  and drop it; never invent build steps.

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
