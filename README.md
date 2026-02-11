# Shopware Language Server

A Language Server Protocol (LSP) implementation for Shopware and Symfony development, providing IDE features across PHP, Twig, XML, YAML, JavaScript/TypeScript, SCSS, and JSON files.

## Features

### Symfony Service Support
- Service ID completion in PHP, XML, and YAML files
- Navigation to service definitions from PHP, XML, and YAML
- Service code lens in PHP files showing service usage
- Parameter reference completion and navigation in XML files
- Service tag completion in XML files
- Service class completion in XML and YAML files
- Tag-based service lookup and navigation
- YAML service configuration support with `@service` reference completion

### Twig Template Support
- Template path completion in Twig files (`extends`, `include`, `sw_extends`, `sw_include` tags)
- Template path completion in PHP files (`renderStorefront` method calls)
- Go-to-definition for template paths in Twig and PHP files
- Twig block indexing and tracking with code lens showing block usage
- Twig filter and function completion with snippet support
- Icon name completion for `sw_icon` tags with pack selection
- Icon preview on hover for `sw_icon` tags (shows SVG preview inline)
- Diagnostics for missing icons in `sw_icon` tags

### Twig Block Versioning
- Tracks block content hashes between Storefront and extensions
- Detects outdated block overrides with warning diagnostics
- Code action to add versioning hash comments to overridden blocks
- Code action to show block diff when version is outdated
- Code action to override a Storefront block in an extension
- Hover shows block hash, template path, and update status

### Snippet Support
- Snippet completion in Twig, PHP, and JavaScript/TypeScript files
- Frontend snippets: `{{ 'key'|trans }}` (Twig), `$this->trans('key')` (PHP)
- Admin snippets: `{{ $t('key') }}`, `{{ $tc('key') }}` (Twig), `this.$t('key')` (JS/TS)
- Go-to-definition for snippet keys (shows all locale variants)
- Hover support showing all available translations for a snippet key
- Diagnostics for missing snippets in Twig and JavaScript/TypeScript files
- Code actions to create snippets from diagnostics or text selections

### Route Support
- Route name completion in PHP (`redirectToRoute`) and Twig (`seoUrl`, `url`, `path` functions)
- Go-to-definition for route names
- Find all references for routes

### Feature Flag Support
- Feature flag completion in PHP (`Feature::isActive()`), Twig (`feature()`), and SCSS files
- Go-to-definition for feature flags

### System Config Support
- System config key completion in PHP (`SystemConfigService::get()`, `getInt()`, `getString()`, `getFloat()`, `getBool()`, `set()`, `getDomain()`)
- System config key completion in Twig (`config()` function)
- Go-to-definition for system config keys

### Theme Config Support
- SCSS variable completion from theme configuration (prefixed with `$`)
- Twig `theme_config()` function key completion
- Go-to-definition for theme config fields

### Admin Component Support
- Component tag completion in administration Twig templates
- Component prop completion with type information, requirements, and defaults
- Slot name completion in `<template #slot-name>` syntax
- Event handler completion (`@event`)
- Parent component name completion in `Component.extend()` calls
- Go-to-definition for component tags, props, slots, and parent components
- Hover showing full component details (props, events, methods, computed properties, slots)
- Diagnostics for missing required props and invalid block references
- Diagnostics for non-existent parent components
- Code action to add missing required props with type-appropriate defaults

### Diagnostics

| Diagnostic | Severity | File Types |
|---|---|---|
| Missing snippet keys | Error | Twig, JS/TS |
| Missing icons in `sw_icon` | Error | Twig |
| Missing required component props | Warning | Twig (admin) |
| Invalid block references in component overrides | Error | Twig (admin) |
| Non-existent parent component | Error | JS/TS (admin) |
| Outdated block version hash | Warning | Twig |
| Missing block version comment | Warning | Twig |

### Commands
- `shopware/forceReindex` - Trigger a full re-index of the workspace

## Supported File Types

| File Type | Features |
|---|---|
| PHP (.php) | Completion, go-to-definition, code lens |
| Twig (.twig) | Completion, go-to-definition, hover, diagnostics, code actions, code lens |
| XML (.xml) | Completion, go-to-definition |
| YAML (.yaml, .yml) | Completion, go-to-definition |
| JSON (.json) | Indexed for snippets and theme config |
| JavaScript (.js) | Completion, go-to-definition, hover, diagnostics (admin) |
| TypeScript (.ts) | Completion, go-to-definition, hover, diagnostics (admin) |
| SCSS (.scss) | Completion, go-to-definition |

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
