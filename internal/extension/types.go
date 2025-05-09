package extension

import "path/filepath"

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
	if e.Type == ShopwareExtensionTypeBundle {
		return filepath.Join(e.Path, "Resources", "views")
	}
	return filepath.Join(e.Path, "src", "Resources", "views")
}
