# Shopware Language Server

A Language Server Protocol (LSP) implementation for Shopware development.

## Features

### Symfony Service Support
- Service ID completion in PHP, XML, and YAML files
- Navigation to service definitions from PHP, XML, and YAML
- Service code lens in PHP files showing service usage
- Parameter reference completion and navigation in XML files
- Service tag completion in XML files
- Service class completion in XML and YAML files
- Tag-based service lookup and navigation
- YAML service configuration support for class completion and service references

### Twig Template Support
- Template path completion in Twig files (`extends`, `include`, `sw_extends`, `sw_include` tags)
- Template path completion in PHP files (`renderStorefront` method calls)
- Go-to-definition for template paths in Twig and PHP files
- Twig block indexing and tracking
- Support for Shopware-specific Twig extensions and tags
- Icon name completion for `sw_icon` tags
- Icon preview on hover for `sw_icon` tags (shows SVG preview inline)

### Snippet Support
- Snippet indexing and validation in Twig files
- Snippet completion in Twig files
- Diagnostics for missing snippets in Twig templates
    - Quick Fix to add missing snippets
- Go-to-definition for snippet keys
- Hover support showing all available translations for a snippet key

### Route Support
- Route name completion in PHP (`redirectToRoute` method) and Twig files (`seoUrl`, `url`, `path` functions)
- Go-to-definition for route names
- Route parameter completion

### Feature Flag Support
- Feature flag indexing and validation
- Go-to-definition for feature flags
- Feature flag completion in PHP files

### Diagnostics
- Snippet validation in Twig templates
- Theme icon validation in Twig templates (checks if referenced icons exist)

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