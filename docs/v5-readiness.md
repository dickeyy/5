# Quack v5 Readiness

Verdict: **NOT READY — FINAL P3 INTEGRATION AND EXTERNAL EVIDENCE PENDING**  
Evidence anchor: `integration/qi-2-p2` at `6b999c1`  
Last reconciled: 2026-07-12

This is the final evidence ledger for the backend repository defined by
[`v5.md`](../v5.md). `TODO.md` and [`v5-scope-drift.md`](v5-scope-drift.md) are
supporting inventories; they cannot override the product definition. A pass
requires implementation plus reproducible evidence. Missing credentials,
infrastructure, sanitized data, or authorization is recorded as **NOT
EXECUTED**, not silently deferred and not treated as a pass.

## Requirement matrix

| ID | Applicable `v5.md` requirements | Implementation and evidence at QI-2 | Current status / remaining evidence |
| --- | --- | --- | --- |
| R01 | Main ideas; v4 comparison; guild-owned rules; moderator-applied rules; Discord authority; immutable understandable history; guild isolation; no hard deletion | Canonical template/case models, frozen snapshots, append-only audit, guild-scoped repositories and compatibility migrations; accepted V5-001/V5-001C/QP-A/QP-E evidence | PASS at QI-2; repeat final boundary and migration gates after P3 |
| R02 | Owner/Admin, Manage Guild, Moderate Members, action-specific permissions, live permission refresh, actor/bot hierarchy, all-or-nothing case preflight | Live authorization service, permission matrix/hierarchy tests, route and Discord authorization; accepted V5-003/QP-A/QP-D/QP-E evidence | PASS at QI-2; final security audit pending |
| R03 | Dashboard/Discord share behavior; member-owned access after departure; Discord is not a template builder; HTTP is internal dashboard/adapter surface | Shared services, staff/member registrars, Discord case workflow, OAuth/session boundary; accepted QP-A/QP-B/QP-D/QP-E evidence | PASS at QI-2; final route-policy inventory pending QP-H |
| R04 | Template fields, identity/versioning, exactly one default level, active/archive/restore, policy-only confirmed import/export | Canonical template service/storage/routes, immutable snapshots and migration compatibility; QP-A tests | PASS at QI-2; final contract coverage reconciliation pending |
| R05 | All-time same-template non-voided escalation including new case; starter policy exact thresholds/actions/appeal/notice | Case lock and escalation selection, starter bootstrap and guild lifecycle; V5-002/QP-A tests | PASS at QI-2; real-guild clean-install rehearsal NOT EXECUTED |
| R06 | Guild-unique immutable case number; valid/voided only; corrections through void plus replacement; member privacy-safe valid/voided history | Case service/storage/routes, void/replacement linkage, target-owned projections; V5-001C/QP-A/QP-D tests | PASS at QI-2; final concurrency/storage restoration proof pending QP-G |
| R07 | Five structured context types; member-visible context; live/link evidence snapshot; managed attachment copy and explicit partial-capture behavior | Context/evidence services, message-link parser, immutable evidence rows, managed channel lifecycle, Discord context wizard; QP-A evidence | PASS at QI-2; real Discord attachment rehearsal NOT EXECUTED |
| R08 | Zero/one timeout, kick, ban; exact settings; target safety; safe automatic retry only; manual recovery reauthorizes | Action model, Discord adapter, leased/fenced execution, classified outcomes and recovery services; V5-003/QP-A evidence | PASS at QI-2; final crash/restore evidence pending QP-G and QP-H |
| R09 | At most one accurate post-outcome member notification; no override; failure does not hide case; retry/dismiss/void controls; limited public Discord result | Durable case notification, action outcome rendering, Discord recovery components and limited public views; QP-A/QP-E evidence | PASS at QI-2; controlled real-guild DM/action rehearsal NOT EXECUTED |
| R10 | One case-linked appeal; snapshotted configurable/default questions; member tracking; reopen/request info; atomic accept+void; explicit authorized reversal | Appeal service, migration 0200, member/staff routes, Discord entry/reversal controls, leased outbox; accepted QP-D/QI-2 evidence | PASS at QI-2; final integrated E2E and real-guild rehearsal pending |
| R11 | Permanent complete audit, successful/failed/denied events, all-moderator read, optional redacted mirror; derived non-leaderboard statistics | Immutable audit store/service, filters/redaction, mirror worker and repair, statistics routes; accepted QP-E/QI-2 evidence | PASS at QI-2; final event inventory/security reconciliation pending |
| R12 | Tickets, general logging, and honeypots remain separately configured modules; honeypot alone applies a normal template automatically; utilities do not shape core | Isolated module registry/storage/routes/Discord adapters/workers and normal-case honeypot adapter; accepted QP-C/QP-F/QI-2 evidence | PASS at QI-2; final module migration/runtime rehearsal pending |
| R13 | V4 cases import as historical, readable, non-escalating records; modules own migrations; isolated coexistence; direct punishment commands removed after transition | QI-2 already excludes `v4_import` from escalation and v5 command registry has no direct punishment commands | INCOMPLETE: QP-G importer, ledger, fixtures, coexistence and rollback evidence pending |
| R14 | Every firm boundary: Discord-specific; no cross-guild/template escalation, public automation API, moderator level/reason override, multi-action, severity/weight/window, notes, hard delete, Quack staff roles, Discord template builder, or general logging in audit | Canonical models/migrations and package boundary tests cover most items | PARTIAL: systematic source/API/security scan and QP-H final policies pending |
| R15 | Repository release quality: migrations, real MySQL/Redis, build/test/vet/race, E2E, security, clean install, upgrade, backup/restore, coexistence, shutdown and real-guild checklist | `internal/readiness/v5_rehearsal_test.go`, `scripts/v5-readiness.sh`, and [`v5-rehearsal.md`](v5-rehearsal.md) define reproducible gates | INCOMPLETE: final accepted P3 heads and evidence execution pending |

## Supporting-inventory audit at QI-2

All 122 unchecked `TODO.md` entries were inventoried by concern before P3
integration. The count is a reconciliation queue, not proof that 122 product
features are absent: several entries describe behavior already present at QI-2
but still need exact final-head evidence before their boxes are changed.

| Concern | Unchecked | Final owner/adjudication |
| --- | ---: | --- |
| Product model alignment | 2 | One archive/restore ledger cleanup; one live soft-delete storage drift routed to QP-G |
| Guild setup and settings | 0 | Rehearsal evidence in QP-I |
| Discord identity and permissions | 1 | QP-I permission-matrix evidence reconciliation |
| Templates and escalation | 1 | QP-I final API contract inventory |
| Case creation and history | 2 | QP-I route/concurrency evidence; QP-G storage concurrency where applicable |
| Evidence capture and preservation | 2 | QP-I test inventory plus real-guild NOT EXECUTED adjudication when authorization is absent |
| Discord enforcement actions | 2 | QP-I route/mock evidence reconciliation |
| Member notifications | 1 | QP-I rendering matrix reconciliation |
| Discord moderator experience | 2 | QP-I accepted QP-D/QP-E evidence reconciliation |
| Authentication and backend API | 6 | QP-H endpoint policy/security; QP-I final contract inventory |
| Appeals | 2 | QP-I integrated appeal E2E and notification evidence |
| Audit log and staff statistics | 5 | QP-I complete-event audit against accepted QP-E/QP-G/QP-H heads |
| Optional ticket, logging, and honeypot modules | 0 | QP-I final isolation/rehearsal evidence |
| V4 migration and coexistence | 19 | QP-G implementation; QP-I safe coexistence rehearsal |
| Database and storage reliability | 13 | QP-G implementation; QP-I migration/backup/restore gates |
| Queue, concurrency, and recovery | 14 | QP-G/QP-H implementation evidence; QP-I race/crash/recovery gate |
| Operations, security, and deployment | 20 | QP-H code/runbooks; QP-I adjudicates prohibited infrastructure and external targets |
| Testing and release readiness | 30 | QP-I final automated gates, security audit, rehearsals, matrix, and verdict |

The scope-drift audit has one newly reopened archive-only mismatch: the live
template storage record still carries `gorm.DeletedAt`. Historical migration
records may retain the value, but the live persistence boundary must not expose
soft deletion as a second lifecycle. It remains open pending QP-G acceptance.

## User completion conditions

| Gate | Status at initial QP-I checkpoint |
| --- | --- |
| Every applicable TODO complete or product-reason adjudicated | INCOMPLETE — 122 unchecked entries require post-P3 reconciliation |
| Every material scope-drift item resolved or accepted/documented | INCOMPLETE — v4 import/direct-command transition remains open |
| Every applicable `v5.md` requirement systematically checked | INVENTORIED as R01-R15; final evidence audit pending |
| Every package passed assigned validation | QI-2 and earlier accepted; QP-G/QP-H/QP-I pending |
| Every package completed its single Codex review-and-fix lifecycle | QI-2 and earlier accepted; QP-G/QP-H/QP-I pending |
| No actionable P0/P1 Codex finding | PASS for accepted heads; final packages pending |
| Newly discovered v5 work completed or adjudicated | OPEN through final audit |
| Repository build/test/lint/type/migration/E2E gates pass | INCOMPLETE — final combined head not available |
| Final readiness matrix and supporting evidence | IN PROGRESS in this document |
| Final READY/NOT READY report with exceptions and evidence | NOT YET FINAL |

## External evidence register

| Evidence | State | Exact dependency |
| --- | --- | --- |
| Real MySQL final migration, locking, JSON, constraints and restore | NOT EXECUTED in QP-I | Disposable MySQL DSN supplied as `QUACK_TEST_MYSQL_DSN`; QP-G final migration head |
| Real Redis durability/outage/recovery | NOT EXECUTED in QP-I | Disposable Redis URL supplied as `QUACK_TEST_REDIS_URL`; QP-H final behavior |
| Sanitized representative v4 dry-run and repeat import | NOT EXECUTED | Accepted QP-G importer plus operator-approved sanitized fixture/source |
| Clean install in a new Discord guild | NOT EXECUTED | Explicit authorization and non-production guild/application credentials |
| Timeout, kick, ban, DM, evidence copy, audit mirror and reversal against Discord | NOT EXECUTED | Explicit authorization, safe test members, managed channels, and non-production guild |
| Deployment/container/Compose smoke and release-infrastructure changes | NOT EXECUTED | User authorization; `.github`, Dockerfile, Compose and deployment infrastructure are outside current authority |
| Production rollout or rollback | NOT EXECUTED | Release-owner authorization and production target; prohibited during repository readiness work |

## Evidence log

Evidence commands, timestamps, commit heads, outcomes, review findings, accepted
exceptions, and the final verdict will be appended here only after execution on
the combined QP-G/QP-H head.
