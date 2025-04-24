# Shopware Language Server

A Language Server Protocol (LSP) implementation for Shopware development.

## Features

### Symfony Service Support
- Service ID completion in PHP and XML files
- Navigation to service definitions from PHP and XML
- Service code lens in PHP files showing service usage
- Parameter reference completion and navigation in XML files
- Service tag completion in XML files
- Service class completion in XML files
- Tag-based service lookup and navigation

### Twig Template Support
- Template path completion in Twig files (`extends`, `include`, `sw_extends`, `sw_include` tags)
- Template path completion in PHP files (`renderStorefront` method calls)
- Go-to-definition for template paths in Twig and PHP files
- Twig block indexing and tracking
- Support for Shopware-specific Twig extensions and tags

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