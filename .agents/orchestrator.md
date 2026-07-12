For Quack v5 work, read these files before planning or modifying code:

1. `v5.md` — intended Quack v5 product behavior and requirements.
2. `v5-scope-drift.md` — known differences between the current implementation
   and the v5 product definition.
3. `TODO.md` — currently known remaining implementation work.
4. `docs/exec-plans/active/v5-readiness.md` — execution state, slices,
   dependencies, PRs, review results, validation, and discoveries.

Precedence for product intent:

1. `v5.md`
2. Explicit clarifications recorded in `v5-scope-drift.md`
3. `TODO.md`

Do not treat `TODO.md` as exhaustive. During implementation and validation,
add newly discovered required work when it is necessary for conformance with
`v5.md`.

## V5 execution rules

- The main agent is the v5 orchestrator.
- Before implementation, reconcile the three v5 documents with the current
  repository and create or update the v5 readiness execution plan.
- Divide work into bounded implementation slices with explicit acceptance
  criteria, dependencies, affected areas, and validation commands.
- Prefer slices that can be reviewed and merged independently.
- Do not run parallel write-heavy slices against overlapping files or APIs.
- Parallelize read-only analysis freely.
- Delegate implementation slices to subagents.
- Assign each implementation slice to a fresh subagent. One subagent owns one
  slice only; do not reuse a prior slice owner for a later task.
- Each implementation subagent owns one branch, one isolated worktree, and one
  pull request.
- A subagent must not merge its own pull request.
- A slice is not complete merely because its implementation is pushed.

## Required implementation-slice lifecycle

Every implementation slice must complete this lifecycle:

1. Read the relevant requirements and inspect the existing implementation.
2. Restate the slice’s acceptance criteria.
3. Implement the smallest coherent change satisfying those criteria.
4. Add or update automated tests.
5. Run all slice-specific checks and relevant repository-wide checks.
6. Review the resulting diff locally.
7. Commit and push the branch.
8. Open a pull request containing:
    - requirement references,
    - implementation summary,
    - validation performed,
    - known limitations,
    - newly discovered follow-up work.
9. Request Codex review using a separate PR comment containing exactly:
   `@codex review`
10. Poll GitHub approximately every 2 minutes until:
    - the Codex review appears,
    - the request clearly failed, or
    - an external blocker requires escalation.
11. Triage every Codex review finding:
    - valid and in scope: fix it;
    - valid but outside the slice: record it in `TODO.md` and report it to the orchestrator;
    - invalid: document the technical reason;
    - ambiguous: investigate before deciding.
12. Push required fixes.
13. Re-run validation after fixes.
14. Do not request another Codex review after the fixes. One Codex review
    request per PR is the default and expected lifecycle.
15. A second `@codex review` request is allowed only when the review fixes are
    a large, substantive rework of the PR (for example, they materially change
    its architecture, public behavior, or most of its implementation). Record
    the reason for the exception in the PR and report it to the orchestrator.
16. Return a structured completion report to the orchestrator.

## Orchestrator validation gate

The orchestrator may validate a slice only after its subagent has completed the
single Codex review-and-fix lifecycle, except for a documented large-rework
second-review exception.

Post-Codex validation is an evidence-focused acceptance gate, not a second full
code-review lifecycle. The orchestrator should verify the subagent's structured
handoff, PR state, Codex result, acceptance-criteria coverage, scope boundaries,
and recorded validation results. Do not routinely repeat a line-by-line diff
review or rerun the complete validation suite when the slice owner already ran
the required commands successfully and Codex reported no bugs.

When all required slice and repository checks pass, the PR has completed its one
Codex review request, Codex reports no actionable bugs, the worktree is clean,
and the handoff accounts for the acceptance criteria and documentation, accept
the slice and advance to the next dependency. PRs remain unmerged, so later
cross-slice or repository-wide validation may harden an accepted slice if new
evidence exposes a problem.

The orchestrator must still independently verify from the submitted evidence:

- acceptance criteria against `v5.md`;
- consistency with adjacent functionality;
- that required validation commands were run and passed;
- PR and Codex review status;
- disposition of every review finding;
- that the PR received no more than one Codex review request unless a documented
  large-rework exception justified a second request;
- absence of unjustified scope expansion;
- documentation and TODO updates;
- whether the slice exposes additional v5 work.

Run additional commands or inspect implementation details only when evidence is
missing, contradictory, a Codex finding required a fix, the change is unusually
high risk, or an acceptance/scope boundary remains uncertain. After review
fixes, target the extra check at the finding and affected behavior; do not ask
Codex for another review or repeat unrelated validation by default.

The orchestrator must reject or return incomplete slices to a subagent rather
than silently repairing them in the orchestration thread.

## Completion criteria

Quack v5 is ready only when:

- every applicable item in `TODO.md` is complete or explicitly documented as
  intentionally deferred with a reason;
- every material discrepancy in `v5-scope-drift.md` is resolved or intentionally
  accepted and documented;
- implementation behavior has been checked against all requirements in `v5.md`;
- all slice PRs completed the single Codex review-and-fix lifecycle, except for
  documented large-rework second-review exceptions;
- all required tests, builds, linting, type checking, migrations, and end-to-end
  validation pass;
- no unresolved P0 or P1 Codex review findings remain;
- newly discovered v5 requirements have been implemented or explicitly
  adjudicated;
- the execution plan contains a final v5 readiness report with supporting
  evidence.

## GitHub behavior

Use the GitHub CLI for pull-request operations and review polling.

Never merge, force-push, delete branches, change repository settings, or modify
release infrastructure unless the active goal explicitly authorizes it.
