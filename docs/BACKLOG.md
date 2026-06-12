# BACKLOG — parked, durable-by-intent

Created by the first coordinator triage (2026-06-12) per the blessed 019
amendment. Governance (encoded here, enforced by the coordinator):
1. **One line per entry**: name | one-line hook | origin pointer | as-of |
   review-by. No bodies, no status, no outcomes — outcomes live in the
   artifacts entries become; lines are DELETED on promote/drop, never
   annotated (`git log -p` is the archive; the file shows NOW, git shows EVER).
2. **review-by defaults +30d.** Coordinator weekly pass: past-date entries are
   re-blessed by the human (date renewed) or become drop-candidates in the
   weekly report — dropped by lazy consent if unclaimed by the NEXT weekly
   (two-week visible window = asynchronous cross-role ack).
3. **Soft cap ~25**; count surfaces in the daily standup mix line; over cap ⇒
   prune pass before any new parking.

| name | hook | origin | as-of | review-by |
|---|---|---|---|---|
| C1 drain best-fit pick | judgment-trial candidate: best-fit vs positional drain pick; sequencing = human 2026-06-19 | inbox draft 019 | 2026-06-11 | 2026-07-12 |
| C2 stale-TAKEN reclaim | judgment-trial candidate: coordinator reclaims dead claims with logged evidence; born from cold-start run 1 | inbox draft 019 | 2026-06-11 | 2026-07-12 |
| C3 drain budget interior | judgment-trial candidate: continue-while-productive inside a hard ceiling (5) | inbox draft 019 | 2026-06-11 | 2026-07-12 |
| C4 session-snapshot content | judgment-trial candidate: judgment-picked snapshot content, scored by next cold-start run | inbox draft 019 | 2026-06-11 | 2026-07-12 |
| P1 token efficiency | audit → options → self-evolving measure; cherry-pick from existing skills (rtk, caveman) | inbox A01 | 2026-06-11 | 2026-07-12 |
| P2 memory measure | preserve-vs-loss measure, visualisation, evaluation (run 1 scored 5.5/6; program stays open) | inbox A01; docs/e2e/cold-start-continuity.md | 2026-06-11 | 2026-07-12 |
| P3 multi-loom fail-fast | multiple looms/projects on one host-user — fail-fast + early; what can be mocked? (next bite, human-ranked) | inbox A01 | 2026-06-11 | 2026-07-12 |
| P4 context-window inspector | visual context-usage presentation; links to P2 | inbox A01 | 2026-06-11 | 2026-07-12 |
| P5 security self-audit | can auditing self-evolve? links to P11 | inbox A01 | 2026-06-11 | 2026-07-12 |
| P6 exhaust-before-prompt | agent exhausts solution space before prompting; human-profile auto-resolve; rubber-stamp bottleneck | inbox A01 | 2026-06-11 | 2026-07-12 |
| P9 windows early deploy | make windows-dev topology real, early (pwsh bootstrap amendment named in SPEC-verbs) | inbox A01; docs/TOPOLOGY.md | 2026-06-11 | 2026-07-12 |
| P10 mobile p2p | mobile app with p2p file sharing — charter/non-goals scope question first (next bite 2, human-ranked) | inbox A01 | 2026-06-11 | 2026-07-12 |
| P11 host security map | color-coded host risk map; kin of spec-map, links to P5 | inbox A01 | 2026-06-11 | 2026-07-12 |
| R1 audit fail-open + tamperable | exec/shell audit appends swallowed on error; actions.log agent-rewritable inside RW mount | docs/reviews/phase-1-review.md (M5+F3) | 2026-06-12 | 2026-07-12 |
| R2 unpinned provisioning | curl-pipe-sh installers + checksum-less Go tarball run as root at build | docs/reviews/phase-1-review.md (M6) | 2026-06-12 | 2026-07-12 |
| R3 drain prompt-injection surface | inbox body echoed verbatim into continuation instructions; orphan guard substring-weak | docs/reviews/phase-1-review.md (M7) | 2026-06-12 | 2026-07-12 |
| R4 inert declared flags | build --stack/--overlay and detect --emit-playbook/--migrate accepted and ignored (vs ErrNotImplemented posture) | docs/reviews/phase-1-review.md (F4) | 2026-06-12 | 2026-07-12 |
| R5 FR↔spec joint nearly unfailable | anchor checked as substring anywhere in spec file; cannot catch miscited/moved clauses | docs/reviews/phase-1-review.md (F5) | 2026-06-12 | 2026-07-12 |
| R6 build result noop unreachable | frozen --json enum value never emitted; agent can't tell did-nothing from reconciled | docs/reviews/phase-1-review.md (F6) | 2026-06-12 | 2026-07-12 |
| R7 engine-level teardown gate | level validation + consent live only in cobra; engine API removes container unconditionally (Phase-2/5 reuse surface) | docs/reviews/phase-1-review.md (F7) | 2026-06-12 | 2026-07-12 |
| R8 doctor outside conformance nets | not in FR-INV-001 covers; no JSON shape block in SPEC-verbs#doctor | docs/reviews/phase-1-review.md (F8) | 2026-06-12 | 2026-07-12 |
| R9 stack knowledge hardcoded | sourcePolicy/goModule/provisionScript/containerHome in Go switches, not config tree — Phase-2 second-stack seam | docs/reviews/phase-1-review.md (F9) | 2026-06-12 | 2026-07-12 |
| R10 flips.log + snapshot hygiene | two state machines in one log, no flag/role field; stale session snapshots assert dead facts | docs/reviews/phase-1-review.md (LOW-7, MED-4) | 2026-06-12 | 2026-07-12 |
