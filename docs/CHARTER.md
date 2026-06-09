# Loom — Charter

## What Loom is

Loom builds and maintains **two-tier, AI-first development environments**. You
declare what an environment should be in a versioned **playbook**; Loom's build
engine reconciles a real environment to match — a shared **base** layer plus
per-project **overlays**, one container per project, devcontainer-compatible at
the floor.

A Loom environment is designed to be operated by a human, by an AI coding agent,
or — eventually — by an AI agent **with no human in the loop**.

## The problem

Setting up and maintaining a capable dev environment is repetitive, easy to get
subtly wrong, and hard to keep reproducible as tools, stacks, and workflows
change over time. Existing tools each solve part of this:

- **Dev Containers** standardize the container but model no policy/intent and no
  machine-wide tier.
- **Devbox/Nix** give reproducible toolchains but carry a learning curve and no
  notion of workflow rules or AI operating context.
- **Ansible** manages config imperatively, but isn't a dev-environment onboarding
  tool and defers container definition to devcontainer anyway.

None of them treat an **AI agent as a first-class user** of the environment, nor
carry the *policy and intent* an autonomous agent needs to operate safely.

## Goals

**Goal 1 — One installer, many situations, no information loss.**
A single, menu-driven, situation-detecting entry point that handles: a fresh
machine (non-technical comfort), an established machine needing a clean reset
without losing scattered credentials, and a load-on-demand cloud sandbox. Getting
back to "working like before" with no major loss is the bar.

**Goal 2 — General environment + per-project overlay, least-confusing model.**
A stable build engine consuming a versioned playbook (base + stack + overlay).
Container-per-project isolation so different stacks never collide. The overlay is
declared in the project's playbook, not applied ad-hoc.

## North star (constrains every decision)

Loom environments must be operable by an autonomous AI agent. Concretely, every
feature must pass: **"could an agent do this unattended, and would the guardrails
hold if it tried the worst thing?"** This implies, as hard requirements:

- Machine-readable (`--json`) output on every verb.
- Idempotent, declarative reconcile (desired-state, not imperative steps).
- Self-describing environments (an agent can query what/why/rules/state from files).
- Guardrails enforced by mechanism (hooks, deny-lists, gates), not by trust.
- Auditable actions (every detect/plan/change leaves a reviewable trail).

## Scenarios

1. **New machine.** One run, minimal edits (keys), no terminal expertise assumed.
   Clear, simple steps.
2. **Established machine, clean reset.** Keys/config scattered across Keychain,
   shell exports, config files, env vars. Detect and carry forward without loss.
3. **Cloud dev-sandbox, on demand.** Ephemeral VM; durable state must live outside
   the VM (volume/secret store/git). A sibling track, not a mode of the local
   installer — but shares the same config internals.

## Non-goals (for now)

- Not a Nix replacement or a general package manager.
- Not a cloud platform/PaaS; the cloud track provisions VMs, it doesn't host apps.
- Not a devcontainer competitor — devcontainer is an **input** we import and
  enrich, never a format we degrade to.
- Not a CI platform; Loom emits CI config, it doesn't run pipelines.

## Audiences

- **Human, non-technical** (scenario 1): gentle, guided, menu-driven.
- **Human, technical** (scenarios 2–3): subcommands, flags, `--json`, escape hatches.
- **AI agent**: structured I/O, self-description, idempotence, enforced guardrails.

One tool, three interfaces over shared internals.

## Success looks like

- A fresh machine reaches a working, AI-capable environment in one guided run.
- An established machine resets cleanly with zero credential loss.
- Two projects of different stacks run side-by-side with no collision.
- An AI agent can `detect → plan → build → update` an environment unattended,
  and the guardrails demonstrably stop destructive actions.
- Loom is built using Loom (dogfooded from the first runnable slice).
