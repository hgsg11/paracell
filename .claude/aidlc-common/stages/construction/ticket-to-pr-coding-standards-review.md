---
slug: ticket-to-pr-coding-standards-review
number: 3.8
name: Ticket Coding Standards Review
plugin: ticket-to-pr
phase: construction
execution: ALWAYS
condition: Executes after the security review passes.
lead_agent: aidlc-quality-agent
support_agents: []
mode: inline
produces:
  - ticket-to-pr-coding-standards-review-report
consumes:
  - artifact: ticket-to-pr-security-review-report
    required: true
  - artifact: requirements
    required: true
requires_stage:
  - ticket-to-pr-security-review
scopes:
  - ticket-to-pr-work-item
inputs: Implemented source diff, repository standards, approved requirements, and prior review report
outputs: ticket-to-pr-coding-standards-review-report.md (under this stage's record dir, engine-resolved)
---

# Ticket Coding Standards Review

## Steps

### Step 1: Review the Change

Inspect the current branch diff and relevant repository guidance, such as
project instructions, formatter/linter configuration, and nearby code. Check
naming, formatting, error handling, test conventions, layering, and consistency
with established patterns. Limit findings to actionable violations in changed
code; do not impose personal style preferences or recommend broad rewrites.

Do not modify source files. Verify any claimed lint or format result by running
the repository's documented command when available.

### Step 2: Record Findings

Write `ticket-to-pr-coding-standards-review-report.md` with verdict `PASS` or
`CHANGES REQUIRED`, each finding's file and line, the violated repository rule
or concrete inconsistency, and a specific correction. Include checks run and
limitations. Use `PASS` only when no actionable coding-standards findings
remain.

If changes are required, present the findings for correction and rerun this
review after the changes. Do not advance with an unresolved finding. Once the
verdict is `PASS`, run `{{INVOKE}} engine orchestrate report --stage ticket-to-pr-coding-standards-review --result awaiting-approval`, present the report, and record the person's response through the orchestrator.

## Sensors

This stage produces `ticket-to-pr-coding-standards-review-report.md` under its
engine-resolved record directory.

## Learn

When the active protocol enables learnings, follow its learning instructions.
