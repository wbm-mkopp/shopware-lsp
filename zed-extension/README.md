# Shopware Language Server for Zed

Zed extension providing [Shopware LSP](https://github.com/shopwareLabs/shopware-lsp) support for PHP, Twig, XML, YAML, JSON, SCSS, JavaScript, and TypeScript files.

## Features

- **Completion** – Services, routes, templates, snippets, feature flags, system config, theme config
- **Go-to-definition** – Jump to definitions for services, templates, snippets, routes, and more
- **Hover** – Documentation and translations on hover
- **Diagnostics** – Missing snippets, icons, component props, outdated block hashes
- **Code actions** – Add versioning hash, extend block in extension, create snippet (when Zed supports them)
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

## Reindexing

The Shopware Language Server watches the workspace and reindexes automatically when project files change. To force a full reindex, reload Zed or disable and re-enable the Shopware LSP extension.

## Extend Twig Block

When viewing a block in `vendor/shopware/storefront` or a `vendor/store.shopware.com` plugin template, place the cursor on the block name and run **Toggle Code Actions** (`cmd-.` / `ctrl-.`). Choose **Extend block '…' in …** for your target extension.

The LSP applies a workspace edit that creates or updates the override template with `{% sw_extends %}` and inserts an empty `{% block %}`. Zed and VS Code open the modified file when the edit is applied.

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
| Force reindex | No in-editor trigger; reindex is automatic on file change, reload Zed to force a full rebuild |

## Troubleshooting

- **LSP not starting**: Check Zed log (`zed: open log`) for errors
- **Debug output**: Run Zed from terminal with `zed --foreground` for verbose logging
- **Build from source**: Place `shopware-lsp` in project root or parent directory

## License

[MIT](../../LICENSE)
