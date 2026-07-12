# Quack v5 Readiness Execution Plan

Status: ACTIVE  
Owner: v5 orchestrator  
Authoritative product definition: [`v5.md`](../../../v5.md)  
Supporting inventory: [`TODO.md`](../../../TODO.md), [`docs/v5-scope-drift.md`](../../v5-scope-drift.md)

## Objective

Bring the backend repository and its Discord and HTTP adapters into conformance
with every applicable rule in `v5.md`, close or explicitly adjudicate every
supporting TODO and scope-drift item, complete the required review lifecycle for
every implementation slice, and publish a final evidence-backed readiness
matrix in `docs/v5-readiness.md`.

## Non-negotiable gates

- `v5.md` wins when code, technical docs, or the backlog disagree.
- Every implementation slice uses an isolated worktree and branch.
- Every slice has explicit acceptance criteria, affected areas, dependencies,
  and validation commands before implementation begins.
- Every slice is committed, pushed, opened as a pull request, and receives an
  explicit standalone `@codex review` request.
- Request Codex review exactly once per PR by default. Apply review findings,
  validate, and hand the fixed head to the orchestrator without another ping.
  A second request is allowed only for documented large substantive rework;
  additional review loops are not an acceptance gate.
- The slice owner triages every Codex finding and completes fixes and
  revalidation before returning the slice.
- Orchestrator acceptance happens only after the review-and-fix lifecycle and
  independent validation.
- Rejected or incomplete work returns to the slice owner; the orchestration
  worktree does not silently repair slice implementation.
- Pull requests are not merged. Branches are not force-pushed. Repository and
  release infrastructure are not changed without explicit authorization.

## Baseline recorded 2026-07-11

- Local base: `main` at `7d12ef3`, one commit ahead of `origin/main`.
- Worktree was clean before the plan was created.
- `TODO.md`: 473 unchecked items across 20 concern areas.
- `docs/v5-scope-drift.md`: 10 material behavior mismatches, 10 rejected
  concepts still represented in code/docs, and 14 missing core capabilities.
- `GOCACHE=/tmp/quack-go-cache go test ./...`: PASS.
- `GOCACHE=/tmp/quack-go-cache go vet ./...`: PASS.
- `GOCACHE=/tmp/quack-go-cache go build ./cmd/quack`: PASS (the generated
  binary was removed).
- GitHub connector: authenticated as `dickeyy`, admin/push access to
  `dickeyy/5`, no open pull requests at baseline.
- Local `gh`: default-sandbox status initially appeared invalid because GitHub
  was unreachable. Escalated `gh auth status` confirmed healthy keyring auth as
  `dickeyy` with `repo` and `workflow` scopes; CLI PR/review operations are
  available outside the restricted network sandbox.

## Integration strategy

The plan will use a cumulative, dependency-ordered branch stack so later work
can be validated with earlier accepted behavior without merging pull requests.
The orchestration branch contains this plan. Each sequential slice starts from
the latest independently accepted slice branch and targets that predecessor in
its PR. Parallel slices are allowed only when their contracts and write sets do
not materially overlap; a dedicated reviewed integration slice must reconcile
parallel branches before dependent work begins.

Planned orchestration branch: `orchestrator/v5-readiness`  
Planned worktree root: `/tmp/quack-v5-worktrees`  
PR base for first slice: `orchestrator/v5-readiness`

## Requirement coverage map

| `v5.md` area | Planned slices | Final evidence |
| --- | --- | --- |
| Main ideas, principles, and firm boundaries | V5-001, V5-001C, V5-004, V5-006, V5-012 | Contract, schema, and boundary tests |
| People and permissions | V5-003, V5-011, V5-014, V5-017 | Permission matrix and hierarchy tests |
| Dashboard, Discord, and backend parity | V5-007, V5-016, V5-017 | HTTP/Discord contract and adapter tests |
| Templates, escalation, starter policy | V5-001, V5-002, V5-004, V5-005 | Template, migration, and bootstrap tests |
| Cases, context, evidence | V5-006 through V5-009 | Atomicity, authorization, capture, and view tests |
| Actions, notifications, recovery | V5-010 through V5-012 | Mock Discord, idempotency, lease, retry, and notification tests |
| Appeals | V5-013, V5-014 | Appeal lifecycle, concurrency, and reversal tests |
| Audit and statistics | V5-015 | Append-only, redaction, filter, mirror, and derived-stat tests |
| Optional modules | V5-018 through V5-022 | Module-specific definitions, isolation, adapter, and migration tests |
| Migration from v4 | V5-023 | Dry-run, idempotency, history, and coexistence tests |
| Operations and release quality | V5-024 through V5-026 | CI, race, integration, smoke, security, and rehearsal evidence |

### TODO ownership

| `TODO.md` concern | Owning slices |
| --- | --- |
| Product Model Alignment | V5-001, V5-001C, V5-004, V5-006 |
| Guild Setup and Settings | V5-002, V5-009, V5-018P |
| Discord Identity and Permissions | V5-003, V5-007, V5-011 |
| Templates and Escalation | V5-004, V5-005 |
| Case Creation and History | V5-006, V5-007 |
| Evidence Capture and Preservation | V5-008, V5-009 |
| Discord Enforcement Actions | V5-010, V5-011 |
| Member Notifications | V5-012 |
| Discord Moderator Experience | V5-009, V5-014, V5-016 |
| Authentication and Backend API | V5-007, V5-017A, V5-017B, V5-017C, V5-017 |
| Appeals | V5-013, V5-014 |
| Audit Log and Staff Statistics | V5-015 |
| Optional Ticket Module | V5-018P, V5-018, V5-019 |
| Optional General Logging Module | V5-018P, V5-020 |
| Optional Honeypot Module | V5-018P, V5-021 |
| V4 Migration and Coexistence | V5-019 through V5-023 |
| Database and Storage Reliability | V5-001M, V5-011, V5-024 |
| Queue, Concurrency, and Recovery | V5-011, V5-012, V5-017C, V5-025 |
| Operations, Security, and Deployment | V5-017B, V5-025 |
| Testing and Release Readiness | every owning slice plus V5-024 through V5-026 |

### Scope-drift ownership

| Drift category | Owning slices |
| --- | --- |
| Escalation windows, enabled states, reason overrides, multi-action settings, mixed validity | V5-001, V5-004, V5-006 |
| Action-specific permission and hierarchy enforcement | V5-003, V5-010, V5-011 |
| Member-owned access | V5-007, V5-017A, V5-017 |
| Import/export/restore/starter policy | V5-002, V5-005 |
| Real actions, recovery controls, and notification | V5-010 through V5-012 |
| Structured context and evidence | V5-004, V5-006, V5-008, V5-009 |
| Appeals, audit, statistics | V5-013 through V5-015 |
| V4 import and removal of direct moderation workflows | V5-016, V5-023 |
| Tickets, general logging, honeypots remain separate modules | V5-018P through V5-022 |

## Dependency waves and slices

Statuses: `PLANNED`, `IN_PROGRESS`, `REVIEW_WAIT`, `FIXING`, `SUBMITTED`,
`ACCEPTED`, `REJECTED`, `BLOCKED`, `DEFERRED`.

### Wave 0 - inventory and orchestration foundation

#### V5-000 - Reconciliation and executable readiness plan

- Status: ACCEPTED
- Branch/PR/review: orchestration worktree; no implementation PR
- Requirements: all of `v5.md`; orchestration protocol
- Acceptance criteria:
  - Every product section maps to one or more planned slices.
  - Every material scope-drift category has an owning slice.
  - All 20 TODO concern groups have an owning slice or final adjudication gate.
  - Baseline tests and external blockers are recorded.
- Write set: this plan only.
- Validation: link/path checks; checklist and coverage audit.
- Evidence: baseline section above; three read-only audits covered core/storage,
  actions/Discord, and auth/API/appeals/audit/modules/operations. Ownership audit
  found 20/20 TODO concern groups and all material scope-drift categories mapped.
  `git diff --check` passed.

### Wave 1 - canonical model and storage boundary

#### V5-001M - Versioned migration foundation

- Status: ACCEPTED
- Assignment: implementation subagent using `v5-implementation-slice`
- Branch: `slice/v5-001m-versioned-migrations`
- Worktree: `/tmp/quack-v5-worktrees/v5-001m`
- Base branch: `orchestrator/v5-readiness`
- Commits: `e67e99a`, `99c018a`, `7608e1a`
- PR/review: [PR #1](https://github.com/dickeyy/5/pull/1); standalone
  `@codex review` posted 2026-07-11; Codex round 1 reviewed `e67e99a` with no
  findings. Orchestrator validation round 1 REJECTED and returned to owner:
  - Real MySQL preservation test fails because the fixture compares source JSON
    text to MySQL's already-normalized stored JSON instead of comparing the
    pre-migration persisted value.
  - P1: the ledger checksum covers only hand-maintained prose, so executable
    migration/schema changes can occur without a checksum mismatch.
  - P1: MySQL DDL implicitly commits, so current Down-plus-ledger-delete
    transaction can leave a partially rolled-back schema recorded as applied;
    no durable rollback-in-progress state or recovery test protects startup.
  Fix commit `99c018a` is pushed with real MySQL validation reported passing;
  Codex round 2 reviewed `99c018a` and raised one P2: frozen migration 0001
  still imports live domain enum types, so later model cleanup could break or
  change the applied migration. Finding was valid/in-scope; fix `7608e1a`
  replaces live aliases with primitive storage types and adds a regression
  guard. Real MySQL/full validation reported passing and the thread was resolved.
  A round-3 request had already been posted before the user clarified the
  single-review policy; it is not awaited or treated as a gate, and no further
  review request will be posted. Future slices follow the updated skill exactly.
- Orchestrator validation round 2: ACCEPTED 2026-07-11.
  - Inspected final diff and migration/CLI/docs/TODO/scope boundaries; no
    unjustified product-model work or unrelated files.
  - `go test -v ./internal/store -run 'TestMySQLMigrat' -count=1` against the
    healthy Compose MySQL: PASS (forward/rerun/preservation/refusal and partial
    DDL rollback dirty-state recovery).
  - `go test ./...`: PASS with loopback permission.
  - `go vet -buildvcs=false ./...`: PASS.
  - `go build -buildvcs=false` for `cmd/quack` and `cmd/quack-migrate`: PASS.
  - `git diff --check`: PASS; worktree clean and synchronized with origin.
  - All orchestrator and Codex findings are fixed; no unresolved P0/P1/P2.
- Requirements: history must remain understandable; important records are not
  hard-deleted; TODO Database and Storage Reliability.
- Acceptance criteria:
  - Production startup runs an ordered, checksum-tracked, idempotent migration
    ledger; `AutoMigrate` is removed from production or explicitly development-only.
  - The current v5 schema upgrades from a representative fixture without losing
    IDs, guild case numbers, snapshots, action attempts, events, or audit rows.
  - Forward, rerun, failure, and rollback procedures are documented and tested
    in SQLite smoke coverage and a real MySQL integration path.
- Dependencies: V5-000.
- Expected write set: migration runner/files, `internal/store/migrations*`, store
  startup/runtime/config, migration tests and documentation.
- Validation: focused store migration tests; MySQL migration/rerun/rollback tests
  when `QUACK_TEST_MYSQL_DSN` is available; repository-wide gates.

#### V5-001 - Simplify the template, level, and action product model

- Status: ACCEPTED
- Assignment: implementation subagent using `v5-implementation-slice`
- Branch: `slice/v5-001-template-model`
- Worktree: `/tmp/quack-v5-worktrees/v5-001`
- Base/PR target: `slice/v5-001m-versioned-migrations`
- Commits: `bf9f204`, `6e5f206` (`bf9f204` was mechanically staged, committed,
  and pushed by the orchestrator after the implementation owner hit an approval
  quota; implementation remained entirely subagent-owned and unchanged).
- Pre-review validation after final soft-delete refinement:
  - focused store/quack/Discord/HTTP tests: PASS;
  - both real-MySQL migration tests: PASS;
  - `go test ./...`: PASS;
  - `go vet -buildvcs=false ./...`: PASS;
  - both application builds and staged `git diff --check`: PASS.
- PR/review: [PR #2](https://github.com/dickeyy/5/pull/2); exactly one
  standalone `@codex review` posted 2026-07-11. Codex reviewed `bf9f204` with
  no findings. Orchestrator validation round 1 REJECTED: migration 0002 archives
  invalid legacy templates but the live get mapper still projects every
  preserved invalid level/action, allowing quarantined templates to expose
  multi-action or invalid-default policy through the v5 response. Returned for
  explicit compatibility-review projection and regression coverage; fixes do
  not receive another Codex request under the single-review policy.
- Orchestrator validation round 2: ACCEPTED 2026-07-11 at remote head
  `6e5f206f6cb6ada44ded9151570659a8df9b6a32`.
  - The compatibility migration now inventories invalid templates even when
    already archived, preserves their source rows, and blocks live projection
    with a typed compatibility-review-required error before levels/actions load.
  - HTTP detail reads return `409 Conflict` with template ID and migration
    reason, without a template or levels payload; valid archived templates
    remain readable.
  - Exactly one `@codex review` request exists on PR #2; it reviewed `bf9f204`
    with no findings. No second request was made after the orchestrator fix.
  - Focused store/quack/Discord/HTTP tests: PASS.
  - Real MySQL `TestMySQLMigrat*` tests: PASS for forward/rerun/preservation and
    partial-DDL rollback recovery.
  - `go test ./...`: PASS with loopback permission.
  - `go vet -buildvcs=false ./...`: PASS.
  - `go build -buildvcs=false` for `cmd/quack` and `cmd/quack-migrate`: PASS.
  - Final fix diff and branch `git diff --check`: PASS; worktree clean and
    synchronized with origin.
  - All Codex and orchestrator findings are resolved; no actionable P0/P1/P2
    finding remains.
- Requirements: `v5.md` Templates, Escalation Levels, Actions, Member
  Notifications, Firm Boundaries; matching scope drift rejected concepts.
- Acceptance criteria:
  - Live template/level/action contracts and snapshots contain no severity,
    escalation window, enabled state, action-level notification, multi-action
    sequencing, `continue_on_error`, or admin-facing backoff/timeout/idempotency
    controls rejected by `v5.md`; only safe retry count remains configurable.
  - A level has exactly zero or one timeout/kick/ban action and exactly one
    default level exists per template; thresholds are positive and unique.
  - Archive is the only template availability signal in live behavior. Existing
    disabled/invalid legacy-v5 configurations are preserved and made safely
    unavailable by an explicit migration rather than silently discarded.
  - Existing v5 rows and historical snapshots remain readable through a new,
    checksum-bound compatibility migration; frozen migration 0001 is untouched.
- Dependencies: V5-001M.
- Expected write set: template/level/action domain and store models/services,
  template HTTP contracts, template-derived case snapshot/action plumbing only
  as needed, migration 0002, focused tests and technical docs.
- Validation: focused template/store/route/case snapshot tests; SQLite and real
  MySQL migration compatibility; `go test ./...`; `go vet ./...`; builds.

#### V5-001C - Simplify case validity, sources, reason, and event model

- Status: PLANNED
- Requirements: `v5.md` Cases, Correcting a Case, Member Access, Firm Boundaries.
- Acceptance criteria:
  - Live case contracts/storage behavior contain no severity, weight, moderator
    reason override, note lifecycle, or mixed action/appeal lifecycle statuses.
  - Case validity is only valid/voided; action and appeal progress remain
    separate. Normal sources are dashboard, Discord, honeypot, and v4 import.
  - Official reason always comes from the immutable template snapshot; adapters
    cannot override it. Free-form note events and note-oriented fields are absent.
  - Existing v5 rows and snapshots are preserved/mapped through a new migration,
    with focused compatibility and API/Discord contract tests.
- Dependencies: V5-001.
- Expected write set: case/event/source domain/store/service/contracts, HTTP and
  Discord case inputs, migrations, affected tests/docs.
- Validation: case/store/route/Discord contract tests; SQLite and real MySQL
  migration compatibility; repository-wide gates.

#### V5-002 - Guild settings, lifecycle bootstrap, and starter policy

- Status: PLANNED
- Requirements: Starter Policy; guild-owned rules; optional-module enablement.
- Acceptance criteria:
  - Guild settings persist core channels, notification branding, module toggles,
    and one-time setup-notice state with Manage Guild authorization and audit.
  - First install creates the exact active, appealable General rule violation
    starter template and thresholds/actions defined by `v5.md`.
  - Guild create/update/leave/rejoin preserves history and repairs stale channel
    references without hard deletion.
- Dependencies: V5-001, V5-001C.
- Expected write set: guild models/repository/service, Discord guild events,
  settings HTTP routes, config/docs/tests.
- Validation: guild/store/route/Discord event tests; exact starter-policy test;
  repository-wide gates.

### Wave 2 - authority, templates, and case transaction

#### V5-003 - Live Discord authorization and target-safety preflight

- Status: PLANNED
- Requirements: People and Permissions; Actions Target Safety.
- Acceptance criteria:
  - Every sensitive write refreshes current Discord membership, permissions,
    actor hierarchy, bot hierarchy, and bot permissions.
  - Owner/Administrator, Manage Guild, Moderate Members, and action-specific
    permission rules match `v5.md`; stored staff rows are attribution caches only.
  - Normal case creation rejects self, bots, Quack, owner, departed members, and
    targets at or above actor/bot hierarchy before any case is committed.
  - Denials use consistent errors and immutable trace-linked audit entries.
- Dependencies: V5-001, V5-001C.
- Expected write set: Discord ports/adapter, guild authorization service, case
  preflight, permission models, tests.
- Validation: exhaustive permission matrix and hierarchy tests; case atomicity
  assertions; repository-wide gates.

#### V5-004 - Structured template context and escalation rules

- Status: PLANNED
- Requirements: Templates, Escalation Levels, Context and Evidence.
- Acceptance criteria:
  - Template contracts persist ordered required/optional short-text, long-text,
    boolean, number, and Discord-message-link fields.
  - Field identifiers, labels, types, order, and values are validated.
  - Escalation uses all-time, same-guild/same-member/same-template-identity,
    non-voided v5 cases and counts the new case; imported v4 history is excluded.
  - Template updates keep identity, increment versions, and immutable case
    snapshots include context definitions and the selected zero-or-one action.
- Dependencies: V5-001, V5-001C.
- Expected write set: template models/service/store/contracts/snapshots/tests.
- Validation: template/context table tests; cross-version escalation and
  concurrency tests; repository-wide gates.

#### V5-005 - Template archive, restore, export, and confirmed import

- Status: PLANNED
- Requirements: Active and Archived Templates; Import and Export.
- Acceptance criteria:
  - Archive is the only availability control; restore is reversible; archived
    templates remain readable and never appear in case creation/autocomplete.
  - Export contains policy only and no guild/channel/history/audit/secret data.
  - Confirmed import validates the payload and creates a new active guild-owned
    identity; all success/failure paths are audited.
- Dependencies: V5-004.
- Expected write set: template service/store, HTTP routes, Discord autocomplete,
  contracts/docs/tests.
- Validation: round-trip, redaction, cross-guild, archive/restore, audit, and
  route tests; repository-wide gates.

#### V5-006 - Atomic case context, validity, void, and replacement

- Status: PLANNED
- Requirements: Cases; Context and Evidence; Firm Boundaries.
- Acceptance criteria:
  - Case creation accepts validated structured context, never a reason override,
    and atomically stores number, snapshots, event, action/notification work,
    and audit after authorization/evidence preflight.
  - Case numbers remain sequential per guild and never reusable.
  - Void requires a reason, appends history, removes the case from escalation,
    and never deletes it; replacement links are explicit and immutable.
  - Action failure never changes case validity.
- Dependencies: V5-003, V5-004.
- Expected write set: case service/store/contracts/routes/tests.
- Validation: atomic rollback, concurrency, void/escalation race, immutable-field,
  replacement, and route tests; repository-wide gates.

### Wave 3 - views, evidence, and enforcement

#### V5-007 - Staff search and privacy-safe member-owned case access

- Status: PLANNED
- Requirements: Dashboard Member Access; Cases Member Access; Audit Log.
- Acceptance criteria:
  - Staff search supports all v5 filters, stable pagination, and summaries.
  - Authenticated targets can list/read only their own cases even after leaving
    or being banned, without relying on current guild membership.
  - Member views hide staff identity and internal errors/payloads/worker/retry
    details while showing reason, visible context/evidence/outcome/history/appeal.
  - Public/staff/internal event projections are explicit and audited.
- Dependencies: V5-006.
- Expected write set: case query service/store, auth/member routes, projections,
  audit hooks, tests/docs.
- Validation: ownership, departed-member, cross-guild enumeration, privacy,
  filters, ordering, and contract tests; repository-wide gates.

#### V5-008 - Evidence snapshots and shared capture service

- Status: PLANNED
- Requirements: Context and Evidence, Capturing Discord Messages.
- Acceptance criteria:
  - Transport-neutral message and attachment snapshots are linked to cases.
  - Shared capture parses Discord links, validates guild/access/target invariants,
    fetches bounded content/metadata, and snapshots before case commit.
  - Deleted/inaccessible/wrong-guild messages produce defined outcomes and never
    silently change the target.
- Dependencies: V5-004, V5-006.
- Expected write set: evidence domain/store/service, Discord evidence port,
  case transaction integration, tests.
- Validation: parser fuzz/unit tests; live/deleted/inaccessible/wrong-guild and
  rollback mocks; limits tests; repository-wide gates.

#### V5-009 - Managed attachment preservation and Discord evidence entry

- Status: PLANNED
- Requirements: Preserving Attachments; Discord message context action.
- Acceptance criteria:
  - Guild setup/repair manages a staff-only evidence channel.
  - Supported attachments are copied with stable references; unsupported or
    oversized copies retain metadata/URL and produce a visible warning without
    blocking otherwise valid case creation.
  - Discord message context and pasted-link flows use the shared capture service
    and collect remaining template context without exposing staff-only channels.
- Dependencies: V5-002, V5-008.
- Expected write set: Discord adapter/interactions/commands/UI, guild channel
  repair, evidence service/store, tests/docs.
- Validation: mocked Discord channel/upload/context-action tests; privacy and
  partial-capture tests; repository-wide gates.

#### V5-010 - Real Discord timeout, kick, and ban execution

- Status: PLANNED
- Requirements: Actions and Action Settings.
- Acceptance criteria:
  - Discord action port and adapter execute timeout, kick, and ban with exact
    configured duration/deletion values and case-number/official-reason audit text.
  - Validation, permission, hierarchy, unknown-member, rate-limit, server,
    timeout, network, and ambiguous outcomes are classified and redacted.
  - Only known-safe outcomes are auto-retryable; uncertain irreversible kick or
    ban outcomes are never automatically retried.
- Dependencies: V5-001, V5-003, V5-006.
- Expected write set: Discord action port/adapter, action executors/service,
  attempt projections, tests/docs.
- Validation: complete mocked Discord outcome matrix, exact-setting assertions,
  timeout/redaction tests; repository-wide gates.

### Wave 4 - recovery, notification, and appeals

#### V5-011 - Action claim leases and staff recovery controls

- Status: PLANNED
- Requirements: Action Failures and Recovery.
- Acceptance criteria:
  - Claims use bounded leases/fencing so stale workers cannot complete reclaimed
    work and crash-left running rows recover safely.
  - Failed review supports query, retry, dismiss, and void without deleting
    history; manual retry re-runs live permission/hierarchy preflight.
  - Timeout removal and unban are explicit staff-confirmed reversals attached to
    the original case/accepted appeal.
  - Every attempt/control/result is idempotent, audited, and observable.
- Dependencies: V5-003, V5-006, V5-010.
- Expected write set: action models/store/service, workqueue, HTTP controls,
  Discord reversals, ops metrics, tests.
- Validation: lease/fencing/crash/race/idempotency tests; retry safety and
  permission tests; repository-wide gates plus targeted `go test -race`.

#### V5-012 - Exactly-once structured member notification

- Status: PLANNED
- Requirements: Member Notifications.
- Acceptance criteria:
  - One case-level notification record replaces `send_dm` actions and all
    action-level notification controls.
  - DM preparation occurs before kick/ban when required; one structured message
    is sent after the enforcement outcome and contains all v5-required fields.
  - Delivery is idempotent and tracked separately; failure never changes case
    validity or causes action retry to resend automatically.
- Dependencies: V5-002, V5-006, V5-010, V5-011.
- Expected write set: notification model/store/service, case transaction,
  action workflow, Discord adapter, settings, tests/docs.
- Validation: rendering matrix; exactly-once duplicate/crash/retry integration
  tests; DM-failure tests; repository-wide gates.

#### V5-013 - Case-linked appeal domain and member/staff API

- Status: PLANNED
- Requirements: Appeals.
- Acceptance criteria:
  - Guild-configured simple questions and a default form are snapshotted per
    appeal; exactly one appeal exists per eligible case.
  - Only the target identity can submit/read; departed/banned users remain
    eligible; voided/non-appealable/unrelated cases are rejected.
  - Request-info/reopen/accept/reject/close use one immutable timeline; member
    views hide staff identities; acceptance atomically voids the case.
  - Reads/writes/decisions are audited and concurrency-safe.
- Dependencies: V5-002, V5-006, V5-007.
- Expected write set: appeal/settings domain/store/service, member/staff routes,
  projections/audit/tests/docs.
- Validation: ownership/state/concurrency/atomic-void/contract tests;
  repository-wide gates.

#### V5-014 - Appeal notifications, Discord views, and reversals

- Status: PLANNED
- Requirements: Appeals notification entry and staff-confirmed reversal.
- Acceptance criteria:
  - Eligible case notifications contain secure appeal access.
  - Staff can inspect appeal status/history through Discord; members receive
    request-info and decision notifications without staff identity leakage.
  - Accepted appeals offer, but never silently execute, timeout removal/unban;
    reversal permission/hierarchy and failure history match v5 rules.
- Dependencies: V5-011, V5-012, V5-013.
- Expected write set: Discord interactions/UI, notification service, reversal
  integration, tests.
- Validation: Discord component/link/view tests; accepted/rejected/reopened/
  failed-reversal end-to-end tests; repository-wide gates.

### Wave 5 - audit, adapter completeness, and API security

#### V5-015 - Complete audit, mirror, redaction, and staff statistics

- Status: PLANNED
- Requirements: Audit Log; Staff Statistics.
- Acceptance criteria:
  - Defined append-only actions cover every meaningful success, denial, failure,
    read, system/import/honeypot action, and recovery operation with correct source.
  - All moderators can search/filter/page the guild audit; secrets and excess
    personal/transport data are redacted.
  - Important events mirror non-blockingly to the configured staff channel and
    channel drift is repairable.
  - Guild-scoped statistics derive from cases/actions/appeals/audit without
    leaderboards or a second source of truth.
- Dependencies: V5-002 through V5-014.
- Expected write set: audit/stat services/store/routes, mirror worker/Discord
  adapter, call-site audit hooks, tests/docs.
- Validation: append-only/redaction/permissions/filter/order/mirror-failure and
  statistics tests; repository-wide gates.

#### V5-016 - Complete Discord moderator case workflows

- Status: PLANNED
- Requirements: Discord; Discord Case Responses; Action Failures and Recovery.
- Acceptance criteria:
  - `/case add` uses active-template autocomplete and structured context, with
    private validation errors and public limited summaries updated after async work.
  - `/case view`, `/case list`, `/case user`, pagination, failed-action review,
    retry/dismiss/void modal, appeal views, and audit mirror are real handlers.
  - Interaction IDs are deduplicated; placeholder handlers and legacy direct
    punishment commands/collisions are absent.
- Dependencies: V5-005 through V5-015.
- Expected write set: Discord commands/interactions/UI/cache/tests/docs.
- Validation: definitions, permissions, autocomplete, modal, pagination,
  component, dedupe, deferred-edit, public/private, and recovery tests;
  repository-wide gates.

#### V5-017A - OAuth and session lifecycle

- Status: PLANNED
- Requirements: Discord-authenticated dashboard/member access.
- Acceptance criteria:
  - OAuth refresh or stable forced-reauthentication handles expired/revoked
    grants without internal errors; logout and compromise revoke sessions.
  - Session IDs and OAuth tokens are never returned in public JSON or logs;
    production cookie attributes are explicit and tested.
  - Former staff lose protected access on the next request while historical
    attribution remains intact.
- Dependencies: V5-003, V5-007.
- Expected write set: auth/session domain/store, OAuth routes/middleware,
  Discord OAuth adapter, config/tests/docs.
- Validation: expiry/refresh/revocation/logout/cookie/redaction and former-staff
  tests; repository-wide gates.

#### V5-017B - HTTP security and stable error baseline

- Status: PLANNED
- Requirements: HTTP API is the safe dashboard/internal adapter.
- Acceptance criteria:
  - Configured production CORS fails closed; cookie-authenticated writes have
    CSRF protection; bodies and server read/write/idle phases are bounded.
  - Security headers and stable errors containing code, safe message, request ID,
    and correlation ID are consistent across authentication, authorization,
    validation, conflict, and dependency failures.
  - Raw Discord/service errors, cookies, tokens, and secrets never reach public
    responses or logs.
- Dependencies: V5-017A.
- Expected write set: HTTP server/config, middleware/error package, route error
  adapters, tests/docs.
- Validation: origin/CSRF/body-limit/timeout/header/error/redaction matrix;
  malformed and cross-guild tests; repository-wide gates.

#### V5-017C - Rate limits and write idempotency

- Status: PLANNED
- Requirements: duplicate protection and safe bounded adapter behavior.
- Acceptance criteria:
  - OAuth, member reads, template writes, case creation, retries, and evidence
    capture use documented per-actor/guild limits with deterministic Redis
    unavailable/expiry behavior.
  - Replayed Discord interaction IDs and HTTP idempotency keys return the
    original/in-progress result without a second case, enforcement, notification,
    appeal, or other externally retried write.
- Dependencies: V5-011, V5-012, V5-016, V5-017B.
- Expected write set: Redis limiter/idempotency adapters, HTTP/Discord middleware,
  write-service contracts, tests/docs.
- Validation: concurrent duplicate/restart/poller-overlap/TTL/unavailable-Redis
  tests and endpoint rate matrix; repository-wide gates.

#### V5-017 - Complete and document the dashboard-facing HTTP API

- Status: PLANNED
- Requirements: HTTP API plus all dashboard/member/staff/admin contracts.
- Acceptance criteria:
  - All required staff/admin/member/settings/module routes expose consistent
    structured errors, IDs, pagination, authorization, and bounded bodies.
  - Production CORS, CSRF, secure cookies, session revocation/refresh failure,
    rate limits, server timeouts, security headers, redacted logging, and write
    idempotency are implemented and fail closed.
  - Dashboard-facing contracts are documented; malformed/oversized/expired/
    revoked/cross-guild cases have complete tests.
- Dependencies: V5-002 through V5-016, V5-017A, V5-017B, V5-017C.
- Expected write set: config, HTTP server/middleware/routes, auth/session store,
  logging, contract docs/tests.
- Validation: full route contract/security matrix; repository-wide gates.

### Wave 6 - optional modules, isolated from moderation core

#### V5-018P - Optional-module definitions and shared registry

- Status: PLANNED
- Requirements: Optional Modules and their separation from the moderation core.
- Acceptance criteria:
  - Module-specific product documents settle settings, lifecycle, permissions,
    retention/privacy, rate limits, Discord behavior, failure recovery, and v4
    migration for tickets, general logging, and honeypots within `v5.md` bounds.
  - A shared per-guild module registry/settings boundary lets modules enable and
    configure independently without fields or special cases in the core case model.
  - Runtime/router/Discord extension points allow later module slices to own
    isolated packages and migrations without overlapping shared registries.
- Dependencies: V5-002, V5-015, V5-017.
- Expected write set: module product docs, shared module settings/registry,
  extension interfaces, settings/status API skeleton, tests.
- Validation: independent enablement, guild isolation, registry extension, and
  core-model boundary tests; repository-wide gates.

#### V5-018 - Ticket module definition and backend lifecycle

- Status: PLANNED
- Requirements: Optional Modules - Tickets; firm separation boundary.
- Acceptance criteria:
  - A module-specific definition records settings, permissions, lifecycle,
    transcript retention, privacy, and Discord behavior without changing core cases.
  - Per-guild settings and open/resolve/cancel/reopen, event timeline,
    transcript, duplicate/rate controls, audit, and authorized API are complete.
- Dependencies: V5-018P.
- Expected write set: ticket-specific docs/domain/store/service/routes/tests only,
  plus shared module registry interfaces.
- Validation: lifecycle/privacy/permission/transcript/API/isolation tests;
  repository-wide gates.

#### V5-019 - Ticket Discord adapter and v4 ticket import

- Status: PLANNED
- Requirements: Tickets private Discord support threads; module migration.
- Acceptance criteria:
  - Discord entry, private thread/channel, queue/view/reply/close, permission
    repair, and deleted-channel behavior conform to the module definition.
  - Ticket import is dry-run, idempotent, audited, and independent of core
    historical-case import.
- Dependencies: V5-018.
- Expected write set: ticket Discord adapter/interactions/UI/import/tests/docs.
- Validation: Discord permission/lifecycle/recovery and migration fixture tests;
  repository-wide gates.

#### V5-020 - General logging module

- Status: PLANNED
- Requirements: Optional Modules - General logging; separation from audit.
- Acceptance criteria:
  - Module definition fixes event, routing, privacy, formatting, retention, cache,
    and retry boundaries.
  - Per-guild configured message/member/moderation/server events deliver to
    staff-only channels with bounded cache/retry and redaction, never the audit log.
  - Settings/status API, repair behavior, audit of configuration, and dry-run
    idempotent v4 settings migration are complete.
- Dependencies: V5-018P.
- Expected write set: logging-specific docs/domain/store/service/Discord events/
  cache/routes/migration/tests, plus shared module registry interfaces.
- Validation: cache/event/format/permission/privacy/retry/repair/migration and
  core-audit isolation tests; repository-wide gates.

#### V5-021 - Honeypot module

- Status: PLANNED
- Requirements: Optional Modules - Honeypots.
- Acceptance criteria:
  - Module definition fixes trigger/exemption/safety semantics.
  - Per-guild configured channel and active template create system-attributed
    cases through the normal transaction, escalation, action, notification,
    evidence, and audit path with bot/staff exemptions and loop prevention.
  - Archive/channel drift disables safely; settings/status/statistics and dry-run
    idempotent v4 migration are complete.
- Dependencies: V5-005, V5-006, V5-009 through V5-017, V5-018P.
- Expected write set: honeypot-specific docs/domain/store/service/Discord events/
  routes/migration/tests, plus explicit system case-actor boundary.
- Validation: setup/exemption/loop/action/failure/drift/migration and normal-path
  integration tests; repository-wide gates.

#### V5-022 - Optional-module integration and isolation audit

- Status: PLANNED
- Requirements: Optional Modules boundaries.
- Acceptance criteria:
  - Tickets, logging, and honeypots enable independently per guild.
  - Module settings, data, events, failures, migrations, and permissions cannot
    contaminate core case/appeal/audit semantics or another guild/module.
  - Runtime intent selection and graceful shutdown cover only enabled features.
- Dependencies: V5-019, V5-020, V5-021.
- Expected write set: module registry/runtime/config/integration tests/docs.
- Validation: cross-module/guild isolation, lifecycle, intent, shutdown, and
  migration integration tests; repository-wide gates.

### Wave 7 - migration, operations, and release evidence

#### V5-023 - V4 historical-case import and coexistence

- Status: PLANNED
- Requirements: Migration From v4.
- Acceptance criteria:
  - Dry-run/idempotent import preserves useful history and source identity,
    labels records historical, excludes them from escalation/actions/DMs, and
    exposes them only through authorized staff/member history.
  - Batches record checksums/counts/warnings/failures and audit safely without
    leaking member data.
  - v4/v5 schemas, Redis keys, processes, and command scopes remain isolated;
    transition/rollback removes direct commands after migration.
- Dependencies: V5-006, V5-007, V5-015, V5-016, V5-022.
- Expected write set: import domain/store/CLI or operator API, command-scope
  checks, fixtures/tests/docs.
- Validation: malformed/departed/missing-user/action-type fixtures, dry-run,
  repeat/rollback, escalation exclusion, privacy, and collision tests;
  repository-wide gates.

#### V5-024 - Versioned production migrations and storage recovery

- Status: PLANNED
- Requirements: history preservation and operational recoverability.
- Acceptance criteria:
  - Production uses reviewable forward migrations with documented rollback;
    startup AutoMigrate is development-only if retained.
  - Constraints/indexes cover v5 invariants and migrations preserve IDs, case
    numbers, snapshots, actions, appeals, evidence, modules, and audit history.
  - MySQL/Redis durability, cleanup, backup, restore, and duplicate-prevention
    behavior is documented and tested against representative current data.
- Dependencies: all schema-producing slices through V5-023.
- Expected write set: migration framework/files, store startup, integration
  fixtures, backup/restore docs/tests.
- Validation: clean/current-schema forward and rollback; MySQL constraints/
  locks/JSON/transactions; Redis expiry/recovery; restore duplicate tests;
  repository-wide gates.

#### V5-025 - Operations, security, CI, and failure runbooks

- Status: PLANNED
- Requirements: operational quality implied by dependable v5 behavior.
- Acceptance criteria:
  - Liveness/readiness/degraded-guild status, metrics, structured trace-linked
    logs, redaction, startup config validation, graceful shutdown, and operator
    recovery/rollback procedures cover every core/module workflow.
  - CI requires build, test, vet, race, integration, migration, vulnerability,
    secret, and meaningful core coverage gates using the pinned Go version.
  - Container/Compose resource, startup, readiness, OAuth, command sync, ops,
    migration, and shutdown smoke behavior is documented and automated where safe.
- Dependencies: V5-022, V5-024.
- Expected write set: ops/config/runtime/status/metrics/logging, CI, container/
  Compose, runbooks/tests.
- Validation: CI-equivalent local commands; targeted race; Compose smoke;
  redaction/health/shutdown tests; repository-wide gates.

#### V5-026 - End-to-end audit, rehearsals, and final readiness evidence

- Status: PLANNED
- Requirements: every applicable statement in `v5.md`; all completion gates.
- Acceptance criteria:
  - Automated E2E covers template, case, escalation, evidence, enforcement,
    notification, audit, member access, appeal, void, recovery, and modules.
  - Controlled coexistence/rollback, clean install, existing-v5 upgrade, backup
    restore, and real test-guild checklists are executed or an explicit product
    reason and owner accepts a documented deferral where execution is impossible.
  - Every TODO item and scope-drift entry is checked, removed, or explicitly
    deferred with a product reason; no unresolved actionable P0/P1 remains.
  - `docs/v5-readiness.md` maps every requirement to implementation and exact
    evidence, and declares READY or NOT READY with accepted exceptions.
- Dependencies: V5-001 through V5-025 accepted.
- Expected write set: E2E/rehearsal harnesses, TODO/scope drift, all technical
  docs, `docs/v5-readiness.md`.
- Validation: all project gates, clean diff audit, systematic requirement/TODO/
  drift review, PR/review evidence audit.

## Slice ledger

| Slice | Status | Branch | PR | Codex rounds | Orchestrator validation |
| --- | --- | --- | --- | --- | --- |
| V5-000 | ACCEPTED | orchestration worktree | n/a | n/a | passed |
| V5-001M | ACCEPTED | `slice/v5-001m-versioned-migrations` | [#1](https://github.com/dickeyy/5/pull/1) | 1 default + 1 large-rework exception; extra request ignored | passed round 2 |
| V5-001 | FIXING | `slice/v5-001-template-model` | [#2](https://github.com/dickeyy/5/pull/2) | 1 complete, no findings | rejected round 1 |
| V5-001C | PLANNED | pending | pending | 0 | pending |
| V5-002 | PLANNED | pending | pending | 0 | pending |
| V5-003 | PLANNED | pending | pending | 0 | pending |
| V5-004 | PLANNED | pending | pending | 0 | pending |
| V5-005 | PLANNED | pending | pending | 0 | pending |
| V5-006 | PLANNED | pending | pending | 0 | pending |
| V5-007 | PLANNED | pending | pending | 0 | pending |
| V5-008 | PLANNED | pending | pending | 0 | pending |
| V5-009 | PLANNED | pending | pending | 0 | pending |
| V5-010 | PLANNED | pending | pending | 0 | pending |
| V5-011 | PLANNED | pending | pending | 0 | pending |
| V5-012 | PLANNED | pending | pending | 0 | pending |
| V5-013 | PLANNED | pending | pending | 0 | pending |
| V5-014 | PLANNED | pending | pending | 0 | pending |
| V5-015 | PLANNED | pending | pending | 0 | pending |
| V5-016 | PLANNED | pending | pending | 0 | pending |
| V5-017A | PLANNED | pending | pending | 0 | pending |
| V5-017B | PLANNED | pending | pending | 0 | pending |
| V5-017C | PLANNED | pending | pending | 0 | pending |
| V5-017 | PLANNED | pending | pending | 0 | pending |
| V5-018P | PLANNED | pending | pending | 0 | pending |
| V5-018 | PLANNED | pending | pending | 0 | pending |
| V5-019 | PLANNED | pending | pending | 0 | pending |
| V5-020 | PLANNED | pending | pending | 0 | pending |
| V5-021 | PLANNED | pending | pending | 0 | pending |
| V5-022 | PLANNED | pending | pending | 0 | pending |
| V5-023 | PLANNED | pending | pending | 0 | pending |
| V5-024 | PLANNED | pending | pending | 0 | pending |
| V5-025 | PLANNED | pending | pending | 0 | pending |
| V5-026 | PLANNED | pending | pending | 0 | pending |

## Decisions

- `v5.md` optional modules are in scope because it explicitly defines tickets,
  general logging, and honeypots as v5 modules. Utilities listed as future work
  are out of scope and must not shape core models.
- Dashboard application code is out of this repository's implementation scope;
  backend contracts, authorization, and member/staff/admin behavior are in scope.
- Existing v5 data compatibility is required even while rejected product fields
  are retired.
- Versioned production migrations are required by the current readiness backlog.
  The migration ledger lands before product-field retirement; final schema
  constraints and restore evidence land after all schema-producing slices.
- Duplicate escalation thresholds are rejected as ambiguous. Escalation selects
  the highest distinct reached threshold; position is display order, not a
  threshold tie-breaker.
- Canonical new case sources are dashboard, Discord, honeypot, and v4 import.
  Compatibility mappings preserve existing rows while new contracts stop using
  the current generic source labels.
- Archived templates are included in authorized template lists with explicit
  archived state by default; case-creation selectors and Discord autocomplete
  use active-only queries.
- Discord API-supported action ranges are encoded as typed constants and tested.
  Retry count is the only admin-facing technical control; internal timeout and
  backoff remain bounded application policy.
- A crash after an uncertain irreversible Discord result goes to manual review.
  Quack does not claim cross-system exactly-once delivery and does not
  automatically repeat an ambiguous kick, ban, reversal, or notification.
- Discord public interaction updates are attempted during the bounded interaction
  lifecycle. Durable authorized case views remain the source of truth after an
  interaction token expires.
- Public OAuth responses do not return raw session IDs. Cookie/session and token
  material is never a dashboard JSON contract.
- Cumulative stacked branches preserve the no-PR-merge constraint while allowing
  dependent implementation and final integrated validation.
- Slice review policy follows the updated skill: one `@codex review` request,
  fixes, then orchestrator validation. Only a documented large substantive
  rework can justify one second request.

## Blockers and risks

- RESOLVED 2026-07-11: GitHub CLI authentication and branch push both succeed
  outside the restricted network sandbox. The connector remains an independent
  structured metadata path, while slice PR/review lifecycle uses `gh`.
- BLOCKER: changes to CI workflows, deployment resource limits, and release
  infrastructure are explicitly unauthorized. V5-025 may implement code,
  tests, local checks, and runbooks, but infrastructure edits require explicit
  user authorization or a documented accepted deferral.
- BLOCKER: real-guild Discord checks require a test guild, bot/OAuth credentials,
  and authorization for external moderation/channel changes.
- BLOCKER: representative v4 import, production backup/restore, and rollback
  evidence require sanitized v4 data and operator-provided MySQL/Redis targets.
- RESOLVED 2026-07-11: the existing healthy Compose MySQL can run
  isolated migration databases through its documented development root account.
  V5-001M round-1 failure was fixed; final real-MySQL validation passes.
- RISK: 473 TODO items include manual infrastructure, backup/restore, real-guild,
  and outage rehearsals that require external environments or credentials.
  These remain planned; inability to execute them cannot be silently called a
  pass and must be resolved or explicitly accepted with a product reason.
- RISK: optional-module details are intentionally high-level in `v5.md`.
  Module-specific definitions may choose implementation details only within the
  stated product boundaries; any product-level change requires updating `v5.md`
  first.
- RESOLVED by accepted V5-001M: the cumulative implementation path now uses a
  versioned, checksum-bound ledger with dirty rollback recovery instead of
  startup `AutoMigrate`. V5-024 still owns final constraints and restore evidence.

## Discoveries and adjudications

- 2026-07-11: Baseline repository already passes its three documented local
  release commands, but `docs/release-readiness.md` still explicitly reports
  timeout/kick/ban as unsupported. Owned by V5-010.
- 2026-07-11: Current models still contain every major rejected concept named in
  scope drift (severity, weight, windows, enabled, multi-action plumbing,
  action-level notification, case lifecycle states, and notes). Owned by V5-001.
- 2026-07-11: Current duplicate-threshold tests intentionally assert a
  position-based tie-break that conflicts with the backlog's unambiguous
  threshold rule. Owned by V5-004.
- 2026-07-11: Scope drift overstates restart recovery: pending/retrying action
  rows recover, but running rows have no lease, fencing token, or reclaim path.
  The drift text remains until V5-011 corrects code and documentation.
- 2026-07-11: Current staff routes deny Moderate Members audit access and exclude
  Moderate Members-only staff from dashboard guild listing, contrary to
  `v5.md`. Owned by V5-003 and V5-015.
- 2026-07-11: Appeals and tickets are persistence placeholders only; general
  logging, honeypots, staff statistics, member-owned case APIs, and v4 import
  have no running service implementation. Owned by V5-013 through V5-023.
- 2026-07-11: Existing Discord case tests require the rejected reason override,
  and action tests validate pre-enforcement/generated `send_dm` behavior. Green
  baseline tests therefore prove regression stability, not v5 conformance.
- 2026-07-11: V5-001M Codex round 1 found no issues, but independent validation
  caught a failing real-MySQL test plus checksum-integrity and partial-rollback
  recovery gaps. Review success is not substituted for orchestrator acceptance.
- 2026-07-11: The user clarified the intended single-review lifecycle after PR
  #1 had already received multiple pings. The already-posted extra request is
  documented and ignored as a gate; all future slices request review once,
  apply fixes, and return directly to orchestrator validation.
- 2026-07-11: V5-001 implementation completed while the subagent's Git-write
  approval quota was exhausted. After the user said continue, the orchestrator
  performed only staging/commit/push mechanics and independently reran the
  latest real-MySQL and repository gates; the subagent resumed PR/review ownership.
- 2026-07-11: `TODO.md` is supporting inventory, not proof of incompleteness by
  itself. Items are checked only after implementation and evidence agree with
  `v5.md`; stale or duplicate items will be corrected during owning slices.

## Continuous update protocol

After each assignment, review poll, fix round, or validation gate, update:

1. the slice status and branch/PR columns;
2. validation commands and exact results;
3. Codex findings and dispositions;
4. decisions, blockers, and newly discovered v5 work;
5. the owning items in `TODO.md` and `docs/v5-scope-drift.md` only when evidence
   proves them complete, stale, deferred, or accepted.
