# Repository Instructions

## TDD With TAKT

When using `superpowers:test-driven-development` in this repository, use TAKT workflows instead of the generic manual TDD loop.

For creating or reviewing use case tests, run:

```bash
takt --task "<実装手順>" --workflow usecase-test-review.yaml
```

For writing production code to pass approved use case tests, run:

```bash
takt --task "<実装手順>" --workflow implement-to-pass-usecase-tests.yaml
```

Do not use generic examples such as `npm test` as the primary workflow for this repository.
