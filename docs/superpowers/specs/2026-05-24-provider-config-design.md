# Provider Config Design

## Goal

Make the external implementations used by `pdev` explicit in `.pdev.yml`.

The first supported providers are the implementations already used by the
application:

- source: `git`
- container: `docker`
- session: `tmux`

This change is a configuration foundation. It does not add alternative
implementations yet.

## Configuration

`pdev init` generates a top-level `providers` section:

```yaml
project:
    name: ""
providers:
    source: git
    container: docker
    session: tmux
templates:
    default:
        repository:
            branchPrefix: feat/
            base: main
        containers:
            services: {}
        session:
            windows: []
```

The provider fields are required. A config without `providers`, or with an empty
provider value, is invalid.

## Supported Values

Only these values are valid in this version:

- `providers.source: git`
- `providers.container: docker`
- `providers.session: tmux`

Any other value returns a configuration error. There is no `none` provider in
this version.

## Architecture

Add a domain configuration model:

```go
type ProviderConfig struct {
	Source    string
	Container string
	Session   string
}
```

`domain.Config` gains a `Providers ProviderConfig` field.

The YAML config adapter is responsible for loading and saving the `providers`
section. It should reject missing or unsupported provider values when loading
configuration.

`InitProjectUseCase` creates the default provider config:

```go
Providers: domain.ProviderConfig{
	Source:    "git",
	Container: "docker",
	Session:   "tmux",
}
```

`internal/app` uses the loaded provider config to select adapters. For this
version, selection maps only to the existing adapters:

- `git` -> `source.GitSourceAdapter`
- `docker` -> `container.DockerCLIAdapter`
- `tmux` -> `session.TmuxAdapter`

## Command Behavior

`pdev create` requires a valid provider config because it creates source,
container, and session resources.

`pdev remove` also requires a valid provider config because it removes source,
container, and session resources.

`pdev ls` does not require provider config because it only reads
`.pdev/state.json`.

`pdev init` does not load existing provider config. It creates `.pdev.yml` with
the default provider config.

## Error Handling

Configuration loading fails when:

- `providers` is missing.
- `providers.source` is empty or unsupported.
- `providers.container` is empty or unsupported.
- `providers.session` is empty or unsupported.

Error messages should name the invalid field and value, for example:

```text
unsupported providers.source "svn"
```

For missing or empty fields, use:

```text
providers.source is required
```

## Testing

Add tests for:

- YAML load reads valid provider config.
- YAML load rejects missing `providers`.
- YAML load rejects unsupported provider values.
- YAML save writes the provider config.
- `pdev init` returns default provider config and writes it through the config
  adapter.
- `app.Run` create/remove adapter selection rejects unsupported values.
- `pdev ls` still works without `.pdev.yml` because it does not load provider
  config.
