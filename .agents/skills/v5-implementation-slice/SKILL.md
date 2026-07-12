---
name: v5-implementation-slice
description: Implement one bounded Quack v5 execution-plan slice through PR creation, Codex GitHub review polling, feedback triage, fixes, revalidation, and handoff to the orchestrator. Use only when a concrete slice with acceptance criteria has been assigned.
---

# Quack v5 implementation slice

You are an implementation subagent. Own exactly one execution-plan slice.

## Inputs required

Before editing, identify:

- slice ID and title;
- requirement references;
- acceptance criteria;
- dependencies;
- expected affected code;
- validation commands;
- assigned branch and worktree;
- base branch.

If information is missing, derive it from the execution plan and source
documents where possible. Escalate only when a product decision is genuinely
required.

## Isolation

- Work only in the assigned worktree and branch.
- Confirm the branch and repository state before editing.
- Do not modify files owned by another active slice unless the orchestrator has
  explicitly coordinated the overlap.
- Do not merge the pull request.

## Implementation

1. Read the relevant sections of:
    - `v5.md`
    - `v5-scope-drift.md`
    - `TODO.md`
    - the active v5 readiness execution plan
2. Inspect the existing implementation and tests.
3. Record a concise implementation approach.
4. Implement the slice.
5. Add or update tests.
6. Run focused validation.
7. Run relevant repository-wide validation.
8. Inspect the final diff for unrelated changes.

## Pull request

Commit and push the branch.

Open a pull request with a body containing:

- Slice ID
- Requirement references
- Acceptance criteria
- Summary
- Validation evidence
- Product or engineering decisions
- Known limitations
- Discovered follow-up work

After opening the PR, post a separate comment:

```sh
gh pr comment "$PR_NUMBER" --body '@codex review'
```

Do not rely on text in the pull-request body to trigger the review.

Request Codex review exactly once per PR by default. After the review arrives,
triage all findings, apply required fixes, validate, commit, and push without
posting another `@codex review` comment.

Request a second review only when the fixes are a large, substantive rework of
the PR, such as materially changing its architecture, public behavior, or most
of its implementation. Record the reason for that exception in the PR and
report it to the orchestrator.

## Review polling

Poll GitHub approximately every five minutes.

At each poll, inspect:

- issue comments;
- submitted reviews;
- inline review comments;
- check status.

Example polling structure:

```bash
while true; do
  gh pr view "$PR_NUMBER" \
    --json reviews,comments,statusCheckRollup,headRefOid > /tmp/pr-state.json

  # Inspect whether the requested Codex review has completed.
  # Exit when a Codex review or clear failure signal is present.

  sleep 120
done
```

Use structured JSON rather than scraping formatted terminal output.

Do not treat Codex's reaction to the request as the completed review. Wait for
the actual review result.

Triage

For every Codex finding, record:

- finding reference;
- severity;
- whether it is valid;
- whether it is in scope;
- chosen disposition;
- supporting reason;
- commit or TODO reference when applicable.

Apply valid in-scope fixes. Add valid out-of-scope findings to `TODO.md` and the
execution plan with sufficient context.

After fixes:

1. run focused tests;
2. run applicable repository-wide checks;
3. inspect the diff;
4. commit and push;

Do not request another Codex review after pushing these fixes unless the
documented large-rework exception applies.

## Return format

Return only after the review lifecycle is complete.

Report:

- slice ID;
- branch;
- PR number and URL;
- commits;
- acceptance criteria status;
- validation commands and results;
- Codex review request count and any documented large-rework exception;
- findings and dispositions;
- files changed;
- newly discovered work;
- unresolved blockers;
- recommendation: `READY_FOR_ORCHESTRATOR_VALIDATION` or `BLOCKED`.
