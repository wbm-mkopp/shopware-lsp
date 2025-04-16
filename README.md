# Shopware Language Server

A Language Server Protocol (LSP) implementation for Shopware development.

## Features

- Service ID completion in PHP
- Navigation to service definitions from PHP and XML
- Service code lens in PHP files

## Development

### Requirements

- Go 1.24 or higher

### Building

```bash
go build
```

### Testing

Run the tests with:

```bash
go test ./...
```

Or run tests with race condition detection:

```bash
go test -race ./...
```

### CI/CD

This project uses GitHub Actions for continuous integration:

- Tests are run on every push and pull request
- Code linting is performed using golangci-lint
- Builds are created for verification

## License

[MIT License](LICENSE)