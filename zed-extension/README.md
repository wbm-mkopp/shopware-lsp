# Shopware Language Server for Zed

Zed extension providing [Shopware LSP](https://github.com/shopwareLabs/shopware-lsp) support for PHP, Twig, XML, YAML, JSON, SCSS, JavaScript, and TypeScript files.

## Features

- **Completion** – Services, routes, templates, snippets, feature flags, system config, theme config
- **Go-to-definition** – Jump to definitions for services, templates, snippets, routes, and more
- **Hover** – Documentation and translations on hover
- **Diagnostics** – Missing snippets, icons, component props, outdated block hashes
- **Code actions** – Add versioning hash, extend block, create snippet (when Zed supports them)
- **Code lens** – Block overwrites, goto parent block

## Installation

### From Extensions (when published)

1. Open Zed and go to Extensions (`cmd-shift-x` / `ctrl-shift-x`)
2. Search for "Shopware"
3. Click Install

### As Dev Extension (local development)

1. Build the LSP server: `go build` in the project root
2. Build the extension: `cd zed-extension && cargo build`
3. In Zed: Extensions → "Install Dev Extension" → select the `zed-extension` directory

## Configuration

### Custom Server Path

To use a locally built `shopware-lsp` binary instead of the auto-downloaded one:

1. Place `shopware-lsp` in your project root, or in the parent of the project root
2. The extension will prefer it over the downloaded binary

You can also override the LSP binary in Zed settings:

```json
{
  "lsp": {
    "shopware-lsp": {
      "binary": {
        "path": "/path/to/shopware-lsp"
      }
    }
  }
}
```

### Download Capability

The extension downloads the LSP binary from GitHub releases. If downloads fail, ensure `granted_extension_capabilities` allows it:

```json
{
  "granted_extension_capabilities": [
    { "kind": "download_file", "host": "github.com", "path": ["shopwareLabs", "shopware-lsp", "**"] }
  ]
}
```

## Slash Commands

- `/shopware-restart` – Guidance for restarting the language server
- `/shopware-reindex` – Guidance for forcing a reindex

## Supported Platforms

- macOS (arm64, x64)
- Linux (arm64, amd64)

Windows is not supported by the Shopware LSP binary.

## Known Limitations

Compared to the VSCode extension, some features are limited by Zed's extension API:

| Feature | Status |
|---------|--------|
| Block diff (virtual documents) | Not supported – no `TextDocumentContentProvider` equivalent |
| Snippet creation dialogs | No multi-step input API – use LSP code actions if Zed supports them |
| Code lens / executeCommand | Zed has limited `workspace/executeCommand` support – some commands may not run |
| Restart / force reindex | Slash commands provide guidance; actual restart is via Zed reload |

## Troubleshooting

- **LSP not starting**: Check Zed log (`zed: open log`) for errors
- **Debug output**: Run Zed from terminal with `zed --foreground` for verbose logging
- **Build from source**: Place `shopware-lsp` in project root or parent directory

## License

[MIT](../../LICENSE)
