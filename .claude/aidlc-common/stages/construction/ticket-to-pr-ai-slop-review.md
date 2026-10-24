---
slug: ticket-to-pr-ai-slop-review
number: 3.9
name: Ticket AI Slop Review
plugin: ticket-to-pr
phase: construction
execution: ALWAYS
condition: Executes after the coding standards review passes.
lead_agent: ticket-to-pr-ai-slop-reviewer-agent
support_agents: []
mode: inline
produces:
  - ticket-to-pr-ai-slop-review-report
consumes:
  - artifact: ticket-to-pr-coding-standards-review-report
    required: true
requires_stage:
  - ticket-to-pr-coding-standards-review
scopes:
  - ticket-to-pr-work-item
inputs: Implemented source diff, original ticket, repository instructions, and prior review reports
outputs: ticket-to-pr-ai-slop-review-report.md (under this stage's record dir, engine-resolved)
---

# Ticket AI Slop Review

## Steps

### Step 1: Review the Change

Read the AI Slop Reviewer persona. Inspect the current branch diff and the
original ticket. Check for unsupported assumptions, scope creep, speculative
abstractions or fallbacks, dead code, duplicated validation, invented APIs,
and comments that merely restate the implementation. Ground every finding in
the request, a caller, a repository invariant, or nearby code.

Do not modify source files. Do not label code as slop based on style, author,
or use of AI.

### Step 2: Record Findings

Write `ticket-to-pr-ai-slop-review-report.md` with verdict `PASS` or `CHANGES
REQUIRED`, each finding's file and line, the missing requirement or concrete
redundancy, and the smallest correction. Include limitations. Use `PASS` only
when no evidence-backed findings remain.

If changes are required, present the findings for correction and rerun this
review after the changes. Do not advance with an unresolved finding. Once the
verdict is `PASS`, run `{{INVOKE}} engine orchestrate report --stage ticket-to-pr-ai-slop-review --result awaiting-approval`, present the report, and record the person's response through the orchestrator.

## Sensors

This stage produces `ticket-to-pr-ai-slop-review-report.md` under its
engine-resolved record directory.

## Learn

When the active protocol enables learnings, follow its learning instructions.
