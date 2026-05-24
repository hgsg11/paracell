# Optional Container Provider Design

## Goal

Allow `pdev` to run without a configured container provider.

Source and session providers remain required. Only the container provider becomes
optional.

## Configuration

`pdev init` continues to generate Docker by default:

```yaml
providers:
    source: git
    container: docker
    session: tmux
```

Users may later remove `providers.container` or set it to an empty value:

```yaml
providers:
    source: git
    session: tmux
```

or:

```yaml
providers:
    source: git
    container: ""
    session: tmux
```

Both forms mean that container operations are disabled.

## Behavior

When `providers.container` is omitted or empty:

- `pdev create` still creates the source worktree.
- `pdev create` still creates the tmux session.
- `pdev create` does not create Docker networks or containers.
- `pdev remove` does not remove Docker networks or containers.
- Template `containers.services` entries are ignored.

When `providers.container` is `docker`, existing Docker behavior is used.

When `providers.container` is any other non-empty value, configuration loading
fails with:

```text
unsupported providers.container "podman"
```

## Architecture

Keep `CreateCellUseCase` and `RemoveCellUseCase` unchanged. They should continue
to call the `ContainerPort`.

Add a no-op container adapter in the adapter layer. It implements
`usecase.ContainerPort` and returns nil for create and remove.

Provider selection maps:

- empty container provider -> no-op container adapter
- `docker` -> existing Docker adapter
- unsupported non-empty value -> error

Provider validation changes:

- `providers.source` remains required and must be `git`.
- `providers.session` remains required and must be `tmux`.
- `providers.container` is optional.
- `providers.container` may be empty or `docker`.

## Testing

Add tests for:

- YAML load accepts missing `providers.container`.
- YAML load accepts empty `providers.container`.
- YAML load rejects unsupported non-empty container provider.
- Provider adapter selection returns a non-nil no-op container adapter when
  container is empty.
- `pdev create` with no container provider does not run Docker commands.
- `pdev remove` with no container provider does not run Docker commands.
