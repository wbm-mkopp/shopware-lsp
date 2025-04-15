# Symfony Service Language Server (Go)

A minimal Language Server Protocol (LSP) implementation in Go for autocompleting Symfony service IDs from XML definitions.

## Features
- Parses Symfony XML service files (e.g., `config/services.xml`).
- Provides autocompletion for service IDs in your editor via LSP.

## Usage
1. Build: `go build`
2. Use with an LSP-compatible editor, pointing to the built binary.

## Project Structure
- `main.go`: Entrypoint, starts the LSP server.
- `lsp/server.go`: LSP handlers (initialize, completion, etc).
- `symfony/xml_services.go`: XML service parser.

## Requirements
- Go 1.18+

---
This is a minimal proof-of-concept. Contributions welcome!
