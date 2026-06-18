package twig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	coreStorefrontSrcPath    = "src/Storefront/Resources/views/storefront"
	coreStorefrontVendorPath = "vendor/shopware/storefront/Resources/views/storefront"
	storeShopwareVendorPath  = "vendor/store.shopware.com/"
	storefrontViewsSegment   = "/Resources/views/storefront"
)

type storePluginNamespaceCacheEntry struct {
	namespace string
	mtime     int64
}

var storePluginBundleNamespaceCache sync.Map

func IsOriginalTemplateSource(path string) bool {
	return isCoreStorefrontPath(path) || isStoreShopwarePluginStorefrontPath(path)
}

func isCoreStorefrontPath(path string) bool {
	return strings.Contains(path, coreStorefrontSrcPath) ||
		strings.Contains(path, coreStorefrontVendorPath)
}

func isStoreShopwarePluginStorefrontPath(path string) bool {
	return strings.Contains(path, storeShopwareVendorPath) &&
		strings.Contains(path, storefrontViewsSegment)
}

func ConvertToRelativePath(twigPath string) string {
	index := strings.Index(twigPath, "Resources/views")
	if index != -1 {
		path := strings.TrimPrefix(strings.TrimPrefix(twigPath[index:], "Resources/views"), "/")

		if path == "" {
			return ""
		}

		return formatTwigRelativePath(resolveTwigBundleNamespace(twigPath), path)
	}

	path := strings.TrimPrefix(twigPath, "/")

	if path == "" {
		return ""
	}

	return formatTwigRelativePath(resolveTwigBundleNamespace(twigPath), path)
}

func formatTwigRelativePath(namespace, viewPath string) string {
	if namespace == "" || viewPath == "" {
		return ""
	}

	return fmt.Sprintf("@%s/%s", namespace, viewPath)
}

func resolveTwigBundleNamespace(twigPath string) string {
	if isCoreStorefrontPath(twigPath) {
		return "Storefront"
	}

	if isStoreShopwarePluginStorefrontPath(twigPath) {
		return resolveStorePluginBundleNamespace(twigPath)
	}

	// Local overrides still use @Storefront rel paths so templates extending the
	// same core view share a lookup key in the twig file index.
	return "Storefront"
}

func resolveStorePluginBundleNamespace(twigPath string) string {
	pluginRoot := storePluginRootFromPath(twigPath)
	if pluginRoot == "" {
		return ""
	}

	composerPath := filepath.Join(pluginRoot, "composer.json")
	mtime := composerJSONModTime(composerPath)

	if cached, ok := storePluginBundleNamespaceCache.Load(pluginRoot); ok {
		entry := cached.(storePluginNamespaceCacheEntry)
		if entry.mtime == mtime {
			return entry.namespace
		}
	}

	namespace := bundleNamespaceFromComposer(pluginRoot)
	if namespace == "" {
		namespace = getBundleNameByPath(twigPath)
	}
	if namespace == "" {
		return ""
	}

	storePluginBundleNamespaceCache.Store(pluginRoot, storePluginNamespaceCacheEntry{
		namespace: namespace,
		mtime:     mtime,
	})
	return namespace
}

func composerJSONModTime(composerPath string) int64 {
	info, err := os.Stat(composerPath)
	if err != nil {
		return 0
	}

	return info.ModTime().UnixNano()
}

func InvalidateStorePluginBundleNamespaceCache(pluginRoot string) {
	storePluginBundleNamespaceCache.Delete(pluginRoot)
}

func storePluginRootFromPath(twigPath string) string {
	idx := strings.Index(twigPath, storeShopwareVendorPath)
	if idx == -1 {
		return ""
	}

	fromVendor := twigPath[idx:]
	parts := strings.Split(fromVendor, "/")
	if len(parts) < 3 {
		return ""
	}

	return twigPath[:idx] + strings.Join(parts[:3], "/")
}

func bundleNamespaceFromComposer(pluginRoot string) string {
	data, err := os.ReadFile(filepath.Join(pluginRoot, "composer.json"))
	if err != nil {
		return ""
	}

	var composer struct {
		Extra struct {
			PluginClass string `json:"shopware-plugin-class"`
		} `json:"extra"`
	}

	if err := json.Unmarshal(data, &composer); err != nil {
		return ""
	}

	return bundleNamespaceFromPluginClass(composer.Extra.PluginClass)
}

func bundleNamespaceFromPluginClass(class string) string {
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}

	parts := strings.Split(class, "\\")
	return parts[len(parts)-1]
}

func twigRelPathsEquivalent(a, b string) bool {
	if a == b {
		return true
	}

	aBundle, aView := splitTwigRelPath(a)
	bBundle, bView := splitTwigRelPath(b)

	return twigViewPathsMatch(aView, bView) && strings.EqualFold(aBundle, bBundle)
}

func twigViewPathsMatch(aView, bView string) bool {
	return aView != "" && strings.EqualFold(aView, bView)
}

func twigRelPathLookupKey(relPath string) string {
	bundle, view := splitTwigRelPath(relPath)
	if bundle == "" || view == "" {
		return relPath
	}

	return formatTwigRelativePath(strings.ToLower(bundle), view)
}

func twigRelPathViewLookupKey(relPath string) string {
	_, view := splitTwigRelPath(relPath)
	if view == "" {
		return ""
	}

	return "view:" + strings.ToLower(view)
}

func splitTwigRelPath(relPath string) (bundle, viewPath string) {
	relPath = strings.TrimPrefix(relPath, "@")
	idx := strings.Index(relPath, "/")
	if idx == -1 {
		return relPath, ""
	}

	return relPath[:idx], relPath[idx+1:]
}

func getBundleNameByPath(twigPath string) string {
	index := strings.Index(twigPath, "Resources/views")
	if index != -1 {
		possiblePath := strings.Trim(twigPath[:index], "/")

		if filepath.Base(possiblePath) == "src" {
			return filepath.Base(filepath.Dir(possiblePath))
		}

		return filepath.Base(possiblePath)
	}

	return "unknown"
}
