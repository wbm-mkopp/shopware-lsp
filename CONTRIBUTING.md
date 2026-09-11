# Contributing to the Shopware LSP

Thanks for your interest in contributing.

## Before opening a pull request

Small fixes can go straight to a PR. Examples:

- typo fixes
- broken links
- small documentation improvements
- obvious bug fixes with a clear test or reproduction

For anything larger, please open an issue first and describe what you want to change before starting implementation. This includes:

- new LSP capabilities (code actions, rename, signature help, …)
- support for new file types or new Twig/XML/YAML/PHP constructs
- changes to existing completion, diagnostic, hover, or go-to-definition behavior
- changes to the indexer or its cache format
- larger refactors

This helps us confirm the direction, avoid duplicate work, and keep the project maintainable.

A draft PR is welcome if it helps explain the idea, but feature PRs should generally be discussed before they are reviewed or merged.

## Pull requests

When opening a PR, please include:

- what changed
- why it changed
- how it was tested, including which editor(s) and which project type
  (platform, production template, plugin, app, plain Symfony)
- any related issue or discussion

Please keep PRs focused. Smaller PRs are easier to review and merge.

## Development

Branch from `main` and target `main` with your pull request.

Before submitting, make sure your changes work locally as they should, and run the checks below:

```sh
go test ./...
golangci-lint run ./...
```

Add or update tests for bug fixes and new behavior.

### Using mise

This repository includes a `mise.toml` file for managing the recommended Go
and golangci-lint versions and for providing convenient development tasks.

After installing mise, set up the project tools with:

```sh
mise install
```

You can then run the complete local check suite with:

```sh
mise run check
```

Individual tasks are also available:

```sh
mise run format       # Format Go source files
mise run format-check # Check Go formatting (gofmt and gci) without changing files
mise run build        # Build the shopware-lsp binary
mise run test         # Run the network-isolated test suite (Linux/macOS only)
mise run test-unit    # Run Go tests without the sandbox wrapper (works everywhere)
mise run vet          # Run go vet
mise run lint         # Run golangci-lint
```

For machine-specific settings, create `mise.local.toml`. This file is ignored
by Git and should not contain secrets that belong in a dedicated secret
manager.

### Running your build in an editor

Build the binary and point your LSP client at it instead of the released one:

```sh
mise run build
```

Server logs go to the client's LSP output channel — the VS Code "Shopware Language Server" channel, or `:LspLog` in Neovim.

## Reviews

Maintainers may ask for changes, suggest a different direction, or decline a PR if the approach was not discussed beforehand. That is not personal; it is how we keep the project consistent and sustainable.
