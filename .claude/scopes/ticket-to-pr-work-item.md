---
name: ticket-to-pr-work-item
plugin: ticket-to-pr
depth: Standard
keywords:
  - ticket to pr
  - issue to pr
  - implement github issue
description: Implement a GitHub issue, pass four reviews, and open a pull request
skeleton: off
existing_code: true
runner: true
review_cap: advisory
guard_policy: off
sensors: on
learnings: off
summary_confirmation: on
plan_approval: on
collaborators: off
---

# Ticket to PR scope

Use for a bounded change from a GitHub issue in an existing codebase. The
workflow confirms the ticket's acceptance criteria and implementation boundary,
then implements and verifies the change, performs four independent reviews,
and opens a GitHub pull request.

## Membership

Initialization and reverse-engineering precede Requirements Analysis, where
the ticket is clarified and its implementation brief confirmed. Code Generation
implements the accepted scope and creates tests; Build and Test executes them.
Test review runs first, followed by security, coding standards, and AI slop
reviews. Every review must pass before the pull request stage is reached.
Broader ideation, deployment, and operational stages are skipped for this
ticket-sized change.
