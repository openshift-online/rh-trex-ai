# TRex Framework Assessment — A Critical Companion to the Proposal

**Companion to:** `shareable.html` (this PR) · **Date:** 2026-10-13 · **Status:** Draft for discussion

The proposal in `shareable.html` answers a narrow question: *how should TRex distribute, version, and upgrade itself as a parent framework?* This companion answers the broader one the proposal deliberately set aside: **is TRex a good base to work from, is the approach correct, and where does it sit in the AI-driven SDLC landscape?** It draws on the same evidence — the TRex tree itself, and field study of all three consumers: **hypershell** (live), **agent-control-plane** (retired), and **rh-trex-mgr** (new) — plus the two in-document reviews. Tone: critical but fair, by design.

---

## 1 · How good a base is TRex today?

### What is genuinely good

The core architecture is proven lineage, not invention: Handler → Service → DAO → Model, event flow over PostgreSQL NOTIFY with idempotent handlers, an `init()`-registry plugin system, and the environments framework — the pattern OpenShift-online services have run in production for years. The codegen suite is real engineering, not scaffold theater: a canonical OpenAPI intermediate representation (CG-005) feeding four generators from one normalized front-end; acceptance tests that *build and execute* a generated CLI against a mock HTTP server; characterization gates that must pass unchanged across a parser migration (CG-005's pre-migration gate). Most frameworks in this space have none of that discipline.

### Four criticisms, in order of severity

**1. The ceremony tax is high.** One kind = 16 files, roughly 1,500 lines. For a team that needs the event flow, that is amortized accidental complexity — the whole point of the framework. For "give me a CRUD resource," it is the heaviest CRUD in the Go ecosystem. There is no thin path: no way to declare a simple resource in data and get CRUD without the full plugin apparatus. Telling detail: `rh-trex-mgr` — a TRex child — is building exactly the layer TRex lacks, a data-driven resource-declaration model (`EntityDefinition` + `FieldDefinition` → generated kind). When your child's business domain is your missing feature, the framework has a gap.

**2. The seams are too thin.** Hypershell needed to mount routes at a different prefix — a basic customization — and had to PR the framework (#65). Cross-cutting concerns (telemetry, RBAC middleware) had to be invented as `plugins/otel` and `plugins/rbac` *inside the child*. When a reference customer's ordinary needs become upstream PRs, the seam inventory — not the customer — is the problem.

**3. Identity crisis.** TRex is a *Trusted REST Example* that decided to become a parent platform without finishing the transition. Distribution is broken (four unresolvable tags, a scaffold that cannot resolve its imports — see the proposal, §2.2), and the framework still behaves as though it *is* the application: `pkg/cmd` owns the entry point, auth defaults to Red Hat SSO, database flags default to a `secrets/` directory. Library ambitions, template habits.

**4. The AI-SDLC layer is prose without enforcement.** The specs and skills trees are excellent *content* with no machinery behind them: nothing in CI fails when a skill goes stale or a spec drifts from code. `/reconcile` is only as good as the discipline to run it. TRex ships the industry's best agent-procedure library and gives it no compile step.

---

## 2 · What to add

Beyond shipping the proposal narrowed per Review 2 (table stakes):

- **A thin-child path.** Declare a kind in data → working CRUD, no 16-file ceremony. The TUI's descriptor-driven runtime (CG-006) proves TRex already knows how to be data-driven; extend the pattern to the API surface.
- **Promote the proven cross-cutters.** `plugins/otel` and `plugins/rbac` exist and work — in hypershell. Promote them upstream.
- **Event-broker failure semantics.** Dead-letter handling, retry visibility. Idempotent handlers without failure handling is half a story.
- **Enforcement for the spec/skill layer.** A CI gate that runs the reconcile loop and fails on open gaps — turning the moat into machinery.

---

## 3 · The user experience, measured as a journey

The product is the child engineer's journey, and each stage has a concrete failure today:

| Stage | The experience TRex owes | Today |
|---|---|---|
| **Day 0** | `trex-gen init` produces a project that builds and passes CI | The scaffold cannot resolve its imports (R5) |
| **Day 1** | One command adds a kind; it appears at once in the API, CLI, TUI, and console | Possible only via a checkout + generator script, with framework knowledge |
| **Upgrade day** | One command, per-file conflict report, no checkout, no ref file, no vendored forks | Three hand-maintained pins; vendored generator forks; `--check` targets written in bash |
| **Contribute-back day** | A documented override → PR → release path | Works, but only through personal heroics (PRs #64/#65) |

Hypershell's hand-built bash apparatus is the UX spec: every script they wrote — the ref file, the checkout cache, the drift targets — is a feature TRex owes them (proposal §2.1 maps them one-to-one onto S2/S3).

---

## 4 · Is this the correct approach?

The destination is right; the positioning is undecided. TRex sits in an awkward middle — more opinionated than a library (gorilla/gin and friends), less complete than a platform. The honest competitive set:

- **API-first derivation** (oapi-codegen, TypeSpec, buf/connectrpc): the spec is the contract, everything is derived, you own handlers only. TRex's closest philosophical cousin — it already works this way, just from its own OpenAPI + 16-file kind model.
- **Codegen-first platforms**: go-zero (`goctl`), Kratos (proto-first). go-zero is the most direct competitor — the same bet (generate the boilerplate, own the logic) — and it ships as `goctl install` + one command, which is exactly the proposal's S2.
- **The Rails doctrine**: convention over configuration as an *integrated system*. And here is the uncomfortable truth: the goals stated for this effort — *parent owns all accidental complexity; children keep only essential complexity* — are **platform goals, not library goals**. A library cannot own all accidental complexity; a platform can. TRex should stop apologizing for its opinions and ship as a platform: install, generate, run, upgrade.

On the versioned-parent question specifically, the evidence already ruled: agent-control-plane copy-forked and died at v0.0.31; hypershell hand-built the distribution layer TRex would not ship. The approach is validated by its failures.

---

## 5 · The AI-driven SDLC landscape

The field has converged quickly, and it validates TRex's instincts:

- **Spec-driven development became a movement.** Amazon's Kiro (requirements → design → tasks before code) and GitHub's Spec Kit (`/specify`, `/plan`, `/tasks` slash commands, constitution files) both ship spec-first agent workflows. TRex's specs — RFC 2119 keywords with GIVEN/WHEN/THEN scenarios — are *more* rigorous than either; they have distribution, TRex has depth.
- **Agent contracts standardized.** AGENTS.md (now read by Codex, Cursor, and others), llms.txt, and Anthropic's Agent Skills — which is the SKILL.md format TRex's `skills/` tree already uses. TRex was early.
- **Repo-anchored procedures.** CLAUDE.md files, Cursor rules, hooks, plan modes — the "context files + procedures + CI gates" triad is becoming the standard agent interface to a repository.
- **Verification-driven development.** Tests-first with agents looping against them; characterization tests guarding AI-driven rewrites. CG-005's migration gate is exactly this pattern, applied to a parser.
- **Cloud agents** (Codex, Devin, Factory) reading whole repositories raise the value of what TRex already is: a repo that is *legible to agents*.

---

## Synthesis

The moat is not the framework. go-zero and Kratos will match the codegen; oapi-codegen will match the derivation. The moat is that TRex is the only codebase observed in this space where specs, skills, generators, and CI form **one closed loop** — desired state, procedure, derivation, and verification. Kiro and Spec Kit have the spec layer with no codegen; go-zero has codegen with no spec layer; nobody else has a `/reconcile` that tracks spec↔code gaps across sessions.

Finish the distribution story (this PR), add enforcement so the loop runs itself, and build the thin path for simple resources. Do that, and the "is this the correct approach" question answers itself: **yes — for the audience nobody else is serving, which is teams whose primary developer is an agent.**
