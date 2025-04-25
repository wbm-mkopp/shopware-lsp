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
