# Provider Factory Refactor Design

## Goal

Move provider selection out of `internal/app` and into `internal/usecase`.

`CreateCellUseCase` and `RemoveCellUseCase` will load config, ask a factory for
the concrete `source`, `container`, and `session` adapters, and then execute
their work. `internal/app` becomes wiring only.

## Scope

This refactor targets the factory boundary for providers:

- `container`
- `session`
- `source`

The order matters only for implementation. The resulting API should keep the
three providers independent and testable.

## Architecture

Define a provider factory interface in `internal/usecase`.

The usecase layer owns the contract:

```go
type ProviderFactory interface {
	Source(provider domain.ProviderConfig) (SourcePort, error)
	Container(provider domain.ProviderConfig) (ContainerPort, error)
	Session(provider domain.ProviderConfig) (SessionPort, error)
}
```

`CreateCellUseCase` and `RemoveCellUseCase` gain a `Providers` dependency of
that interface type. After loading config, each usecase resolves the ports it
needs through the factory and then continues as before.

The adapter layer implements the interface in `internal/adapter/provider`.
That implementation keeps the current provider rules:

- `source` must be `git`
- `session` must be `tmux`
- `container` may be omitted or empty and should resolve to a no-op adapter
- `container: docker` resolves to the existing Docker adapter

`internal/app` stops selecting providers directly. It just constructs the
provider factory implementation and passes it into the usecases.

## Behavior

`pdev create` and `pdev remove` still behave the same externally:

- source worktrees are created/removed
- tmux sessions are created/removed
- container operations run only when the container provider resolves to Docker

The refactor changes where the selection happens, not the behavior.

## Refactor Path

Do the provider work one provider at a time:

1. `container`
2. `session`
3. `source`

This keeps each step small and lets the no-op container path stay isolated.

## Error Handling

Provider selection errors remain explicit and field-specific:

- `unsupported providers.source "svn"`
- `unsupported providers.container "podman"`
- `unsupported providers.session "ssh"`

The container provider may be empty. Empty container provider is not an error
and should resolve to the no-op adapter.

## Testing

Add or update tests for:

- `CreateCellUseCase` and `RemoveCellUseCase` resolve ports through a factory.
- `internal/adapter/provider` returns the no-op container adapter when the
  container provider is empty.
- `internal/adapter/provider` still rejects unsupported source and session
  values.
- `internal/app` no longer contains provider-selection logic.
- `go test ./...` still passes after the refactor.
