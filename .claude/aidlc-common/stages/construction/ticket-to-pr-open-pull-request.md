---
slug: ticket-to-pr-open-pull-request
number: 4.0
name: Open Pull Request
plugin: ticket-to-pr
phase: construction
execution: ALWAYS
condition: Executes after all review reports pass.
lead_agent: aidlc-delivery-agent
support_agents: []
mode: inline
produces:
  - ticket-to-pr-pull-request
consumes:
  - artifact: build-test-results
    required: true
  - artifact: ticket-to-pr-security-review-report
    required: true
  - artifact: ticket-to-pr-coding-standards-review-report
    required: true
  - artifact: ticket-to-pr-ai-slop-review-report
    required: true
  - artifact: ticket-to-pr-test-review-report
    required: true
  - artifact: ticket-to-pr-implementation-brief
    required: true
requires_stage:
  - ticket-to-pr-ai-slop-review
scopes:
  - ticket-to-pr-work-item
inputs: Confirmed issue brief, verified test results, and all four passing review reports
outputs: ticket-to-pr-pull-request.md (under this stage's record dir, engine-resolved)
---

# Open Pull Request

## Steps

### Step 1: Check Readiness

Read the implementation brief, Build and Test results, and all four review
reports. Stop and report `failed` if a review verdict is not `PASS`, an
acceptance criterion is unverified, or required tests failed. Confirm that the
current branch contains only the intended ticket change and that the GitHub CLI
is authenticated for the repository. Do not print credentials or tokens.

### Step 2: Prepare the Pull Request

Prepare a concise title and body from the issue and actual diff. Include a
summary, test evidence, links to the four review reports, and a closing
reference such as `Closes #123` when the issue number is known. Do not claim
checks passed unless the recorded evidence supports it.

If changes are uncommitted or the branch has not been pushed, make a focused
commit containing only the ticket's changes and push the current branch so the
requested PR can be opened. Use the repository's established commit-message
convention when one exists. If the branch or repository state makes that
unsafe or ambiguous, stop and explain the specific obstacle.

### Step 3: Create the Pull Request

Create the PR with the repository's GitHub CLI (`gh pr create`) using the
prepared title and body. Record the resulting PR URL. If the command fails,
report the error and do not claim a PR exists.

Write `ticket-to-pr-pull-request.md` with the title, URL, linked issue,
review/test evidence, and any remaining non-blocking limitations. Run
`{{INVOKE}} engine orchestrate report --stage ticket-to-pr-open-pull-request --result awaiting-approval`, present the PR URL and report, and record the person's response through the orchestrator.

## Sensors

This stage produces `ticket-to-pr-pull-request.md` under its engine-resolved
record directory.

## Learn

When the active protocol enables learnings, follow its learning instructions.
