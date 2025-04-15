# Symfony Service Autocompletion for VSCode

This VSCode extension provides autocompletion for Symfony service IDs from XML definitions.

## Features

- Autocompletion for Symfony service IDs in PHP and XML files
- Works with any Symfony project that uses XML service definitions
- Real-time updates when service definitions change
- Supports incomplete/broken XML during editing

## Requirements

- VSCode 1.74.0 or newer
- A Symfony project with XML service definitions

## Extension Settings

This extension contributes the following settings:

* `symfonyServiceLSP.enable`: Enable/disable the Symfony Service LSP
* `symfonyServiceLSP.serverPath`: Path to the Symfony Service LSP server executable. Leave empty to use the bundled server.

## Installation

### From Source

1. Build the language server:
   ```bash
   cd /path/to/shopware-lsp
   go build
   ```

2. Install extension dependencies:
   ```bash
   cd vscode-extension
   npm install
   ```

3. Build the extension:
   ```bash
   npm run compile
   ```

4. Create a symlink to your VSCode extensions folder:
   ```bash
   ln -s /path/to/shopware-lsp/vscode-extension ~/.vscode/extensions/symfony-service-lsp
   ```

5. Restart VSCode

### From VSIX (once packaged)

1. Download the VSIX file
2. In VSCode, go to Extensions view
3. Click "..." in the top-right corner
4. Select "Install from VSIX..."
5. Choose the downloaded VSIX file

## How It Works

The extension uses a Go-based Language Server Protocol (LSP) implementation that:

1. Scans your project for XML files
2. Parses them using Tree-sitter for robust handling of even incomplete XML
3. Extracts Symfony service IDs
4. Provides autocompletion as you type

## Development

To build and test the extension:

```bash
# Build the language server
cd /path/to/shopware-lsp
go build

# Build the extension
cd vscode-extension
npm install
npm run compile
```

## License

MIT
