---
slug: ticket-to-pr-test-review
number: 3.65
name: Ticket Test Review
plugin: ticket-to-pr
phase: construction
execution: ALWAYS
condition: Executes after Build and Test and before the other reviews.
lead_agent: aidlc-quality-agent
support_agents: []
mode: inline
produces:
  - ticket-to-pr-test-review-report
consumes:
  - artifact: requirements
    required: true
  - artifact: build-test-results
    required: true
requires_stage:
  - build-and-test
scopes:
  - ticket-to-pr-work-item
inputs: Acceptance criteria, test source diff, and build/test results
outputs: ticket-to-pr-test-review-report.md (under this stage's record dir, engine-resolved)
---

# Ticket Test Review

## Steps

### Step 1: Review Test Quality

Inspect the tests added or changed for this ticket and the build/test results.
Check that each accepted criterion has a meaningful assertion, tests exercise observable
behavior rather than implementation details, and failure cases are covered
where the criterion requires them. Look for vacuous assertions, tests that
always pass, and mocking that removes the behavior under test.

Do not modify application or test files. Report only actionable findings tied
to an acceptance criterion or a concrete test-quality defect.

### Step 2: Record Findings

Write `ticket-to-pr-test-review-report.md` with verdict `PASS` or `CHANGES
REQUIRED`, each finding's test file and line, the criterion or test property at
issue, and a specific correction. Include checks run and limitations. Use
`PASS` only when every applicable criterion has meaningful test coverage.
If changes are required, present findings for correction, rerun Build and Test,
and rerun this review after the changes. Do not advance with unresolved
findings. Once the verdict is `PASS`, run
`{{INVOKE}} engine orchestrate report --stage ticket-to-pr-test-review --result awaiting-approval`, present the report, and record the person's response through the orchestrator.

## Sensors

This stage produces `ticket-to-pr-test-review-report.md` under its
engine-resolved record directory.

## Learn

When the active protocol enables learnings, follow its learning instructions.
