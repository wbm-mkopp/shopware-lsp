package symfony

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/php/semantic"
)

const symfonyBundleInterface = "Symfony\\Component\\HttpKernel\\Bundle\\BundleInterface"
const maxBundleResourceCandidates = 1_000

var bundleResourceRoots = []string{
	"Resources/config",
	"config",
	"Controller",
	"src/Controller",
}

type routeBundleCatalog struct {
	roots map[string][]string
	names map[string]string
}

type BundleResourceCandidate struct {
	Value string
	Path  string
}

// RouteResourceResolver adds legacy @Bundle resource resolution to the
// filesystem-relative route resource helpers. Its bundle catalog is rebuilt
// only when the immutable PHP workspace generation changes.
type IndexedResourcePaths interface {
	ResourcePaths(context.Context, string, int) ([]string, error)
}

type RouteResourceResolver struct {
	paths    IndexedResourcePaths
	php      *php.PHPIndex
	mu       sync.Mutex
	revision uint64
	catalog  *routeBundleCatalog
}

func NewRouteResourceResolver(
	phpIndex *php.PHPIndex,
	paths ...IndexedResourcePaths,
) *RouteResourceResolver {
	resolver := &RouteResourceResolver{php: phpIndex}
	if len(paths) > 0 {
		resolver.paths = paths[0]
	}
	return resolver
}

func (resolver *RouteResourceResolver) Files(
	currentPath string,
	reference RouteResourceReference,
) []string {
	if !strings.HasPrefix(reference.Path, "@") {
		return RouteResourceFiles(currentPath, reference)
	}
	var result []string
	for _, target := range resolver.bundleTargets(reference.Path) {
		result = append(
			result,
			routeResourceFilesAtTarget(target, reference)...,
		)
		if len(result) >= maxRouteResourceFiles {
			result = result[:maxRouteResourceFiles]
			break
		}
	}
	return uniqueRouteResourcePaths(result)
}

func (resolver *RouteResourceResolver) Matches(
	currentPath,
	candidate string,
	reference RouteResourceReference,
) bool {
	if !strings.HasPrefix(reference.Path, "@") {
		return RouteResourceMatches(currentPath, candidate, reference)
	}
	for _, target := range resolver.bundleTargets(reference.Path) {
		if routeResourceMatchesTarget(target, candidate, reference) {
			return true
		}
	}
	return false
}

func (resolver *RouteResourceResolver) bundleTargets(
	resource string,
) []string {
	name, relative, found := routeBundleResourceParts(resource)
	if !found {
		return nil
	}
	roots := resolver.bundleCatalog().roots[strings.ToLower(name)]
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		target := root
		if relative != "" {
			target = filepath.Join(
				root,
				filepath.FromSlash(relative),
			)
		}
		result = append(result, filepath.Clean(target))
	}
	return uniqueRouteResourcePaths(result)
}

// BundleResourceCandidates returns the same conventional bundle config and
// controller files offered by the reference plugin.
func (resolver *RouteResourceResolver) BundleResourceCandidates(
	ctx context.Context,
) []BundleResourceCandidate {
	catalog := resolver.bundleCatalog()
	if catalog == nil {
		return nil
	}
	keys := make([]string, 0, len(catalog.roots))
	for key := range catalog.roots {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seen := make(map[string]struct{})
	var result []BundleResourceCandidate
	for _, key := range keys {
		name := catalog.names[key]
		for _, root := range catalog.roots[key] {
			for _, conventional := range bundleResourceRoots {
				base := filepath.Join(root, filepath.FromSlash(conventional))
				if resolver.paths == nil || ctx.Err() != nil || len(result) >= maxBundleResourceCandidates {
					continue
				}
				paths, err := resolver.paths.ResourcePaths(ctx, base, maxBundleResourceCandidates-len(result))
				if err != nil {
					continue
				}
				for _, path := range paths {
					relative, err := filepath.Rel(root, path)
					if err != nil {
						continue
					}
					value := "@" + name + "/" + filepath.ToSlash(relative)
					if _, exists := seen[value]; exists {
						continue
					}
					seen[value] = struct{}{}
					result = append(result, BundleResourceCandidate{Value: value, Path: path})
				}

			}
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Value < result[right].Value
	})
	return result
}

func (resolver *RouteResourceResolver) bundleCatalog() *routeBundleCatalog {
	if resolver == nil || resolver.php == nil {
		return nil
	}
	revision := resolver.php.Revision()
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if resolver.catalog != nil && resolver.revision == revision {
		return resolver.catalog
	}

	snapshot := resolver.php.SemanticSnapshot()
	roots := make(map[string][]string)
	names := make(map[string]string)
	for _, symbol := range snapshot.GlobalSymbols() {
		if symbol.Kind != semantic.ClassSymbol ||
			symbol.Path == "" ||
			symbol.Flags.Has(semantic.InternalFlag) ||
			!snapshot.IsSubtypeOf(
				symbol.FullyQualified,
				symfonyBundleInterface,
			) {
			continue
		}
		key := strings.ToLower(symbol.Name)
		names[key] = symbol.Name
		roots[key] = append(
			roots[key],
			filepath.Clean(filepath.Dir(symbol.Path)),
		)
	}
	for name, candidates := range roots {
		roots[name] = uniqueRouteResourcePaths(candidates)
	}
	resolver.revision = revision
	resolver.catalog = &routeBundleCatalog{
		roots: roots,
		names: names,
	}
	return resolver.catalog
}

func routeBundleResourceParts(
	resource string,
) (string, string, bool) {
	resource = strings.TrimSpace(resource)
	if !strings.HasPrefix(resource, "@") {
		return "", "", false
	}
	normalized := strings.ReplaceAll(resource[1:], "\\", "/")
	normalized = strings.TrimLeft(normalized, "/")
	separator := strings.IndexByte(normalized, '/')
	if separator <= 0 {
		return "", "", false
	}
	name := normalized[:separator]
	relative := strings.TrimLeft(normalized[separator+1:], "/")
	if name == "" {
		return "", "", false
	}
	return name, relative, true
}

func uniqueRouteResourcePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}
