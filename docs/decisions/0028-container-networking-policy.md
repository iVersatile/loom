# ADR-0028 — Container networking policy: deny-by-default egress, declared hostname allowlist, provision-then-restrict

**Date:** 2026-06-21   **Status: Accepted** (human, 2026-06-21 — "Accepted, ALLOW_SPEC_CHANGE ADR-0028"; via advisor; T20-successor of the S1 mechanism proof #246). **· Amended 2026-06-21 (Amendment 1, ALLOW_SPEC_CHANGE):** the allowlist mechanism is the **SNI/CONNECT proxy sidecar** (promoted from the deferred S3 to BE S2b), evidence-driven by the real-docker proof #249. The original §3 "IP custom network first" ordering + the §3 iptables-fragility note + the §4 allowlist are superseded by **Amendment 1** (appended below; original decision text retained per the append-only doctrine).

> "Agent drafts, human accepts" (RULES §5 / ADR-0017). Scoping spike +
> archaeology: `.scratch/spikes/t20-egress-control.md`. The mechanism this ADR
> governs was de-risked first as a proof primitive — T20 S1, the internal
> `ContainerSpec.NoEgress` → `--network none` mode + the integration canary
> `TestT20S1NoEgressHasNoNetworkInterface` (#246). This ADR is the policy that
> S1 proved was *enforceable*; the spec freezes here, the useful build (S2) lands
> after acceptance. Spec before code (RULES §2 / ADR-0006).

## Context — the deny-list is TRUST; the fix must be MECHANISM
loom's egress control today is **100% harness-permission deny-list**
(`settings.json`: curl/wget/nc/ssh/`python -c`/`node -e`/…). That is **trust, not
mechanism**: any code that opens a socket routes around it. The allowlist permits
`go test`/`go build` (arbitrary compiled network I/O); the deny-list omits
`python3`/`node`/`perl` *script-file* invocations and bash `/dev/tcp` (phase-1
review H3/HIGH-1). `createRunArgs` (`internal/engine/container.go`) sets no
`--network`, `--dns`, or firewall — every project container gets Docker's default
bridge = **full unrestricted outbound**.

This is the unmitigated exfil path for the credential work: ADR-0026 (gh token)
and ADR-0027 (per-project credentials) both place a **secret at rest in a
per-project volume** and both name **T20 egress as the complementary, BLOCKING
control** — every blast-radius bound they assert is void if a compromised agent
can phone the token home. The ADR-0005 "worst thing" test fails at the network
layer. T20 S1 proved the *mechanism* layer is reachable (`--network none` → a
container with only `lo`, physically unable to exfiltrate); this ADR decides the
*policy* that makes egress restriction **useful for a working agent** rather than
all-or-nothing.

**Principle (carried from the spike):** *policy lives outside the box it polices.*
The agent runs as root inside its container (ADR-0019 is per-project, not
universal) and can undo any in-container fetters — so the control must sit at the
Docker layer the agent cannot reach, never `iptables` inside the container.

## Decision

### 1. A declared egress posture: the `networking:` playbook field
A new optional `networking:` section (SPEC-playbook freeze, the `user:`/`role:`
precedent — see the proposed clause text in §6). It expresses **what the agent may
reach**, declaratively and host-independently, as **hostnames**; the engine
realizes it through whatever mechanism is current (§3). Three postures:

| `egress:` | Meaning | Mechanism |
|---|---|---|
| `off` *(default / unset)* | full outbound — **Phase-1 compatible**: every existing playbook keeps meaning what it meant | Docker default bridge (today) |
| `none` | no egress at all (frozen container / proof / offline batch) | `--network none` — **the T20 S1 primitive, now reachable from the playbook** |
| `allowlist` | deny-by-default; only `allow:` hostnames reachable at runtime | custom network (S2) → proxy sidecar (S3) |

`allow:` is a hostname list. **Unset `networking:` = `off`** — identical to today,
so this clause is purely additive (the `user:` unset=root move).

### 2. Provision-then-restrict (the create-time trade — T20's biggest fork)
Provisioning needs **broad** egress (apt, go.dev module proxy, installers,
github); the running agent needs a **small** allowlist. Docker cannot change a
container's network *mode* live, but it **can** `network disconnect`/`connect` a
container to a custom network at runtime. So:

- **Provision on the default bridge** (full egress) — apt/go/installers work
  exactly as today; the provision sentinel digest is unchanged.
- **Restrict before handing the container to the agent:** disconnect from bridge,
  connect to the deny-by-default loom-managed network whose only reachable
  destinations are the resolved `allow:` set. The agent never sees broad egress.

**Rejected:** recreate-on-policy-change (tear down + rebuild whenever the
allowlist edits). Heavier, and it conflates a *policy* edit with a *provisioning*
event. Provision-then-restrict keeps the two lifecycles separate and lets a policy
change be a cheap reconnect, not a rebuild. (A policy change still rides the
convergence digest so a plain `loom build` re-converges the network binding,
mirroring the home/role sentinels.)

### 3. Mechanism: custom network first (S2), proxy sidecar as the end state (S3)
- **S2 — loom-managed deny-by-default Docker network (IP-level), the next build.**
  Docker-native, cross-platform (mac linuxkit VM / windows WSL2 behave
  identically — favors docker-native, a second strike against in-container
  iptables). Realizes `allow:` by resolving hostnames → IPs at connect time. This
  is the shape the credential build (ADR-0027) needs and the smallest useful
  slice.
- **S3 — egress proxy sidecar (hostname/SNI), the durable end state.** The proxy
  owns the allowlist and is the container's only route out; matches **hostnames**
  at the TLS handshake (no DNS fragility), supports **hot policy edits** without a
  reconnect, and gives observe→enforce. More moving parts (a loom-managed sidecar
  lifecycle). Not the first slice.
- **`in-container iptables` — REJECTED** (agent is root → undoes its own fetters =
  trust not mechanism; cross-platform-fragile).

The `networking:` schema is **mechanism-independent** (it names hostnames, not
IPs or proxy config), so the S2→S3 migration is an engine change behind a frozen
spec, not a spec change.

### 4. The load-bearing allowlist (don't brick the agent)
A deny-by-default that forgets the model endpoint **kills the agent**. The
allowlist's first entries are load-bearing and the **base tier owns them**:
- the **model API** endpoint (`api.anthropic.com`, or the metered key's
  endpoint) — without it the agent cannot think;
- the **credential helper's** egress (the store/refresh endpoint for an
  `apiKeyHelper`/`oauth-file` adapter, ADR-0027);
- VCS over HTTPS (`github.com`) for the git-controller seat (ADR-0026).

Projects **add** their own service hosts; the resolved allowlist is the **union**
across tiers (base load-bearing + project-declared). The provision-time hosts
(apt/go.dev/installers) are **not** in the runtime allowlist — they are reachable
only during the broad-egress provision phase (§2), so the runtime surface stays
minimal.

### 5. Merge semantics (mirror `user:`/`role:`)
- `egress:` is a **last-non-empty-wins scalar** (like `user:`). The
  "a later tier *weakens* egress (`allowlist`→`off`)" edge is **not** special-cased
  in the scalar merge; it is enforced at the **full-auto re-evaluation gate** —
  exactly the precedent set for `user:`'s "a later layer re-grants root" (ADR-0019).
  Guardrails are mechanism: weakening egress is a visible playbook edit (a PR, a
  flip in `flips.log`), never silent.
- `allow:` is a **union** across tiers (base load-bearing entries always present;
  tiers add, never remove) — the `rules:`/`dotfiles:` union precedent, so the model
  endpoint cannot be dropped by an overlay.

## 6. Proposed SPEC-playbook clause text (for the human to authorize verbatim or edit)
> **`networking:` (optional, section, later-wins scalar + union list): the
> container's egress posture.** Unset means `off` — full outbound, the Phase-1
> default (every existing playbook is unchanged). `egress: none` removes all
> network access (`--network none`; no interface but `lo`). `egress: allowlist`
> denies by default and permits only the resolved `allow:` hostnames at runtime.
> `allow:` is a hostname list, authored at any tier; the resolved set is the
> **union** in layer order (the base tier contributes the load-bearing model-API,
> credential-helper, and VCS hosts that a deny-by-default must not drop). The
> `egress:` scalar is last-non-empty-wins (like `user:`); a later tier weakening
> the posture is caught at the full-auto re-evaluation gate, not the scalar merge.
> Provisioning runs with full egress; the restricted network is applied before the
> agent seat opens (provision-then-restrict). The mechanism (custom network /
> proxy sidecar) is an engine concern behind this declarative field.

```yaml
networking:
  egress: allowlist          # off (default) | none | allowlist
  allow:                     # runtime-reachable hostnames (union across tiers)
    - api.anthropic.com      #   base contributes the load-bearing entries
    - github.com
```

## Alternatives considered (rejected)
- **In-container iptables / firewall.** Agent is root → can flush its own rules.
  Trust, not mechanism. Also cross-platform-fragile. (Spike option (c).)
- **Recreate-on-policy-change** instead of provision-then-restrict. Conflates a
  policy edit with a rebuild; heavier; no benefit over a live reconnect. (§2.)
- **IP allowlist authored directly in the playbook.** Breaks on CDN / rotating
  IPs (`api.anthropic.com`, the go.dev module proxy) and is non-portable. We
  author **hostnames**; the mechanism resolves them (S2 at connect time, S3 at
  SNI). (Spike "IP vs hostname".)
- **Keep the harness deny-list as the only control.** The status quo this ADR
  exists to fix — it is trust, bypassable by any socket-opening code (the T20
  premise).
- **Proxy sidecar (S3) as the FIRST slice.** Most parts; not needed to prove the
  shape the credential build requires. Deferred to the end state, not rejected.

## Consequences
**Positive**
- Converts T20's claim ("the deny-list fails at the network layer") into an
  enforceable **mechanism** behind a declarative field; satisfies ADR-0026/0027's
  BLOCKING egress dependency → **unblocks the credential build**.
- Passes the ADR-0005 worst-thing test at the network layer: a compromised agent
  with a token at rest **cannot exfiltrate** to a non-allowlisted host.
- Purely additive + Phase-1 compatible (`unset = off`); mechanism-independent
  schema lets S2→S3 evolve without a spec change.
- Docker-native S2 is cross-platform with no divergence (mac/windows both run the
  Docker-Desktop Linux engine).

**Trade-offs**
- S2's IP-level realization inherits **CDN/rotating-IP fragility** (re-resolve on
  reconnect; a moved IP is a transient miss) — the explicit reason S3 (SNI) is the
  end state, not a nice-to-have.
- A **load-bearing allowlist is a footgun**: a base tier that omits the model
  endpoint bricks every agent. Mitigation: the base-owned load-bearing entries
  (§4) + a `doctor` reachability claim (below).
- One more loom-managed Docker object (the network; later the sidecar) →
  teardown must clean it (rides the existing per-container resource loop).
- Provision-then-restrict adds a connect/disconnect step to the `Ensure` sequence
  — sequencing complexity around `provision()`, not new persistent state.

**Revisit if**
- Multi-tenant/shared hosts need per-flow audit → promote S3 (proxy) from end
  state to required.
- Hostname allowlists prove insufficient (wildcard/SNI-spoof concerns) → the proxy
  gains content/policy inspection.

## Realization (the build this authorizes — slices)
- **S1 ✅ (landed, #246):** internal `ContainerSpec.NoEgress` → `--network none` +
  integration canary. The proof primitive. **This ADR retroactively gives it the
  `networking.egress: none` home** and an FR (`FR-NET-001`, the no-egress mode) —
  S1 shipped without one as a proof primitive; the FR lands with the spec.
- **S2 (next build, after acceptance):** the `networking:` field
  (schema/merge/validate) + the deny-by-default custom network + provision-then-
  restrict sequencing + the base load-bearing allowlist + a `doctor`
  `container:egress` reachability claim (model endpoint reachable / a
  non-allowlisted host not) + integration canary on a real restricted container.
  FRs: `FR-NET-002` (allowlist posture), `FR-NET-003` (provision-then-restrict),
  `FR-NET-004` (doctor egress claim). The Go plumbing + FRs are autonomous; the
  `networking:` SPEC clause is the trust-path piece this ADR carries.
- **S3 (end state):** the egress proxy sidecar (hostname/SNI, hot policy,
  observe→enforce). Its own follow-up; same frozen `networking:` schema.

## Supersedes / relationships
Supersedes nothing. Builds on **T20 S1** (#246, the proven primitive) and the
scoping spike. **Unblocks ADR-0027** (per-project credentials — the BLOCKING
egress control they name) and complements **ADR-0026** (the gh token at rest, whose
mitigation it explicitly couples to T20). Realizes the network-layer fix the
**ADR-0005** worst-thing test demands. Schema follows the **ADR-0019** `user:`/
`role:` freeze + merge precedent. Mechanism rejects in-container iptables on the
**ADR-0019** non-root-where-declared + "policy outside the box" grounds.

---

## Amendment 1 (2026-06-21) — the allowlist mechanism is the egress PROXY, not an IP network

**Accepted** (human, 2026-06-21 — "accept / ALLOW_SPEC_CHANGE — merge it and build S2b"; via advisor).
Evidence: the 3-agent confer `.scratch/spikes/t20-s2b-allowlist-mechanism.md` + the **real-docker proof
#249** (`TestT20S2bProxyEgressAllowlist`, integration tier GREEN — all three assertions). The original
decision (above) leaned **S2 = a docker-native IP custom network first, S3 = an egress proxy sidecar
later**; this amendment revises the *mechanism and its sequence*. **The policy is unchanged.**

### Why (the evidence)
1. **The "IP custom network" mechanism does not exist as a docker primitive.** A custom/`--internal`
   network gives deny-*all*, not deny-by-default-with-exceptions; the per-host exceptions can come only
   from host iptables or a proxy — so §3's "S2 = loom-managed deny-by-default IP network" had no concrete
   docker-native realization.
2. **Host iptables (`DOCKER-USER`) is Linux-only** — on Docker Desktop (Mac LinuxKit / Windows WSL2) the
   daemon's netfilter lives in a VM the host cannot reach; mac-dev is loom's only *validated* topology.
3. **The load-bearing allowlist is ~80% rotating-CDN hosts** (Go→Google, apt→Fastly, npm/astral→
   Cloudflare, PyPI/githubusercontent→Fastly, LFS→S3, telemetry→GCP); only ~5 (Anthropic-owned + github)
   have stable CIDRs. An IP allowlist either breaks on rotation or admits whole CDN ranges. **It fights
   the data.**
4. **The proxy is the only per-host AND cross-platform AND not-trust mechanism — and it is PROVEN** (#249):
   a project container on an `--internal` network with a sidecar gatekeeper as its sole route → an
   allowlisted host reachable *through* it, a non-allowlisted host blocked (403), and a **direct bypass
   dead** (the `go test`/`/dev/tcp` exfil path T20 exists to close). provision-then-restrict demonstrated.

### The three revisions
- **R1 — Mechanism ordering (supersedes §3).** **S2b = the SNI/CONNECT egress-proxy sidecar** (was the
  deferred "S3"). The project container is on an internal network whose **only** route off-box is a
  loom-managed sidecar running a per-HOSTNAME allowlist proxy (the route is the fence; `HTTPS_PROXY` is
  convenience for cooperating clients, NOT the control — a raw socket has nowhere to go). Lean:
  **explicit-CONNECT** (no TLS decryption / no MITM CA; immune to ECH); a small loom-shipped Go proxy
  (no third-party image in the trust path). The docker-native IP-network "S2" is **dropped** (Why-1).
- **R2 — Correct the iptables-fragility note (supersedes the §3 Alternatives wording).** ADR-0028
  labeled in-container iptables "cross-platform-fragile" — **backwards**. *Host* DOCKER-USER iptables is
  the Linux-only one; *in-container* iptables is cross-platform-robust (same kernel everywhere). Under
  T10 non-root it is a genuine candidate, but it stays IP-level (the CDN fragility, Why-3) and hinges on
  dropping CAP_NET_ADMIN from the agent's `docker exec` (docker exec has no `--cap-drop` — an unproven
  unknown). So it remains **not chosen**, on the *correct* grounds (IP-level + the cap-drop unknown), not
  the mis-stated cross-platform one. The proxy is the portable mechanism.
- **R3 — Correct the load-bearing allowlist (supersedes §4's 3 entries).** The runtime allowlist MUST
  also include **claude.ai + console.anthropic.com** (OAuth login / token refresh — ADR-0014/0027 M1;
  **absent from Anthropic's own devcontainer firewall**, a known footgun) and very likely
  **statsig.anthropic.com / statsig.com / sentry.io** (CLI feature-gates + error reporting). The base
  tier owns these; the engine **unions a hardcoded load-bearing set into any declared allowlist** so a
  forgotten host can never brick the agent. `container:egress` doctor = a hard pre-flight; first
  deployment is **observe→enforce** (log would-be-denies before enforcing — a capability the proxy has).

### What does NOT change
- The **policy**: deny-by-default, declared **hostnames**, provision-then-restrict, "policy outside the
  box," the worst-thing test at the network layer.
- The **`networking:` SPEC-playbook schema** (S2a): unchanged — it names hostnames, mechanism-independent,
  so promoting the proxy is an **engine choice, not a spec change** (this amendment touches only this ADR;
  any S2a clause phrase that called `allowlist` "deferred" is corrected to "supported as of S2b").
- **S1** (`egress: none`) and **S2a** (the field + the `allowlist` validate fail-close): unaffected; S2b
  flips the fail-close to wired. provision-then-restrict survives but **simplifies** (one union allowlist
  → policy reload; the docker network swap becomes optional).

### Updated realization (supersedes the §Realization S2/S3 rows)
- **S2b (the build this authorizes):** an engine-managed proxy sidecar (loom-shipped Go CONNECT proxy) +
  a shared internal network (the project container's sole route) + flip S2a's `egress: allowlist`
  fail-close → wired + the hardcoded load-bearing union (R3) + a `container:egress` allowlist doctor
  claim + observe→enforce. FRs: **FR-NET-002** (allowlist enforced, bypass dead), **FR-NET-003**
  (provision-then-restrict / union-then-narrow), **FR-NET-004** (doctor egress reachability). Proven-out
  by #249, generalized to loom's own sidecar.
- **S3 is absorbed into S2b** — the proxy is the mechanism, not a deferred end-state. Future hardening
  (transparent routing, hot multi-policy, content inspection) layers on the same schema.

### Biggest risk (carried)
**Brick-the-agent via an incomplete runtime allowlist** (R3). Mitigations: the hardcoded base-owned
load-bearing union, the `container:egress` doctor pre-flight, and observe→enforce on first deployment.
