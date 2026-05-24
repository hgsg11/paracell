# Provider Factory Split Refactor Design

## Goal

Split provider factory responsibilities by provider type and move provider
selection out of `internal/app`.

`CreateCellUseCase` and `RemoveCellUseCase` will each ask the usecase-layer
factory interfaces for the concrete `source`, `container`, and `session`
adapters they need. `internal/app` becomes wiring only.

## Scope

This refactor targets the provider factory boundary for:

- `container`
- `session`
- `source`

The order matters for implementation only. The public behavior should stay the
same.

## Architecture

The usecase layer owns three provider-specific factory interfaces:

```go
type SourceProviderFactory interface {
	Source(provider domain.ProviderConfig) (SourcePort, error)
}

type ContainerProviderFactory interface {
	Container(provider domain.ProviderConfig) (ContainerPort, error)
}

type SessionProviderFactory interface {
	Session(provider domain.ProviderConfig) (SessionPort, error)
}
```

`CreateCellUseCase` and `RemoveCellUseCase` depend on the factory interfaces
they actually need. After loading config, each usecase resolves the required
ports and then continues as before.

The adapter layer implements the interfaces in `internal/adapter/provider`.
Provider rules stay the same:

- `source` must be `git`
- `session` must be `tmux`
- `container` may be omitted or empty and should resolve to a no-op adapter
- `container: docker` resolves to the existing Docker adapter

`internal/app` stops selecting providers directly. It constructs the provider
factory implementation and passes it into the usecases.

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

This keeps each step small and keeps the no-op container path isolated.

## Error Handling

Provider selection errors remain explicit and field-specific:

- `unsupported providers.source "svn"`
- `unsupported providers.container "podman"`
- `unsupported providers.session "ssh"`

The container provider may be empty. Empty container provider is not an error
and resolves to the no-op adapter.

## Testing

Add or update tests for:

- `CreateCellUseCase` and `RemoveCellUseCase` resolve ports through provider factories.
- `internal/adapter/provider` returns the no-op container adapter when the container provider is empty.
- `internal/adapter/provider` still rejects unsupported source and session values.
- `internal/app` no longer contains provider-selection logic.
- `go test ./...` still passes after the refactor.
