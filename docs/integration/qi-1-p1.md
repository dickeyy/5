# QI-1 P1 integration manifest

QI-1 combines the accepted QP-B HTTP/auth platform and QP-C tickets/logging
module heads on the accepted V5-003 anchor. It remains intentionally uncommitted
and without a pull request until the accepted QP-A head is incorporated.

## Installed contracts

- `httpapi.NewPlatformRegistrar` owns trusted proxies and the global request,
  error, recovery, logging, security-header, CORS, body-limit, and CSRF order.
- Optional-module HTTP routes are mounted beneath authenticated, live guild
  context and reuse QP-B's Redis rate-limit and idempotency primitives.
- One module registry constructs ticket and general-logging services from the
  process database; the core repository is adapted only as an append-only audit
  sink.
- Ticket components share the command dispatcher. Gateway handlers feed a
  separate bounded logging queue and perform deleted-channel repair.
- Ticket transcript cleanup starts promptly, runs hourly, and stops with the
  process. Shutdown drains accepted logging work without entering the moderation
  action queue.
- Logical module migrations 0100 and 0110 are represented as frozen,
  checksum-bound central-ledger migrations. Their temporary physical positions
  are 0005 and 0006 until QP-A's reserved 0005-0049 range is merged.

## QP-A merge plan

1. Merge the accepted QP-A head into `integration/qi-1-p1` without rewriting
   either package head.
2. Reconcile `internal/quack/app.go` by preserving QP-A core services; optional
   modules stay process-composed and do not enter the moderation core.
3. Preserve QP-A's Discord moderation client additions. QI-1's ticket and
   logging transport remains in `internal/moduleintegration`, so only command
   component registration should require registry reconciliation.
4. Preserve QP-A's migration files and order in reserved range 0005-0049, then
   renumber the two QI-1 physical ledger entries immediately after the final
   QP-A migration while retaining logical identities 0100 and 0110. Update
   filenames, embedded sources, tests, and checksum evidence together.
5. Reconcile `runtime.go`, `router.go`, and command registration once, then run
   the wave-wide focused, race, MySQL, full test, vet, and binary build gates.

No QP-B or QP-C feature implementation is duplicated in this integration
package. Real Discord permission-changing checks remain a release validation
gate requiring authorized credentials.
