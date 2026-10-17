---
name: request-to-issue
description: Use when a user wants to solve a problem, fix a bug, or add a feature and the outcome should be a GitHub issue whose body becomes the single source of truth for design, implementation direction, and acceptance criteria.
---

# request-to-issue

Create a GitHub issue from the current conversation by running design, planning, and issue drafting in order, with the issue body as the single source of truth.

## When to Use

- The user is describing a problem to solve
- The user wants a feature or behavior change
- The user wants the result turned into a GitHub issue
- The current conversation is the primary source of truth

Do not use this when the user already has a complete issue body and only wants it posted.

## Required Dependencies

- **REQUIRED SUB-SKILL:** Use `superpowers:brainstorming` first
- **REQUIRED SUB-SKILL:** Use `superpowers:writing-plans` for planning structure, but do not persist a separate plan document
- **REQUIRED REFERENCE SKILL:** Use `takt-plan-to-issue` for the issue body structure and writing rules

## Workflow

1. Start from the current conversation, not from a fixed artifact file.
2. Read enough local context to understand the affected project and existing conventions.
3. Ask clarifying questions one at a time until the scope, constraints, and success criteria are concrete.
4. Follow the `superpowers:brainstorming` flow to present the design and get approval, but do not persist a separate spec file.
5. Use the structure of `superpowers:writing-plans` to derive an implementation approach, file scope, and verification steps, but do not persist a separate plan file.
6. Convert the approved design and implementation approach into a GitHub issue body using the existing `takt-plan-to-issue` template and section rules.
7. Create the issue with `gh issue create`.

## Output Rules

- The GitHub issue body must contain enough design, implementation direction, and acceptance criteria that work can proceed without separate `.md` documents.
- The issue title and body must follow the existing `takt-plan-to-issue` template rather than inventing a new structure.
- The issue body should be written in Japanese unless the user explicitly asks for another language.

## GitHub Issue Creation

Create the issue only after the draft title and body are coherent with the approved design and implementation approach.

Use `gh issue create` with the final title and body. If GitHub CLI auth or creation fails, stop and present the final draft so the user can create it manually.

## Failure Handling

- If the request is too broad, split it into the first implementable issue before planning.
- If requirements are still ambiguous, keep asking one question at a time instead of drafting around guesses.
- If local project context contradicts the request, call out the contradiction before writing the issue.
