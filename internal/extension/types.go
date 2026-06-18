package extension

import (
	"path/filepath"
	"strings"
)

const (
	customPluginsPath       = "/custom/plugins/"
	customStaticPluginsPath = "/custom/static-plugins/"
	vendorPathSegment       = "/vendor/"
	srcPathSegment          = "/src/"
)

type ShopwareExtensionType int

const (
	ShopwareExtensionTypeBundle ShopwareExtensionType = iota
	ShopwareExtensionTypeApp
)

type ShopwareExtension struct {
	Name string
	Type ShopwareExtensionType
	Path string
}

func (e ShopwareExtension) GetStorefrontViewsPath() string {
	path := strings.TrimSuffix(e.Path, string(filepath.Separator)+e.Name+".php")
	return filepath.Join(path, "Resources", "views")
}

// IsLocal reports whether the extension lives in a project-owned path (src,
// custom/plugins, or custom/static-plugins) rather than vendor.
func (e ShopwareExtension) IsLocal() bool {
	return IsLocalExtensionPath(e.Path)
}

func IsLocalExtensionPath(path string) bool {
	path = filepath.ToSlash(path)
	if strings.Contains(path, vendorPathSegment) {
		return false
	}

	return strings.Contains(path, customPluginsPath) ||
		strings.Contains(path, customStaticPluginsPath) ||
		strings.Contains(path, srcPathSegment)
}
