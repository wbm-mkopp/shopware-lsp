package extension

import "path/filepath"

type ShopwareExtensionType int

const (
	ShopwareExtensionTypeBundle ShopwareExtensionType = iota
	ShopwareExtensionTypeApp
)

type ShopwareExension struct {
	Name string
	Type ShopwareExtensionType
	Path string
}

func (e ShopwareExension) GetStorefrontViewsPath() string {
	if e.Type == ShopwareExtensionTypeBundle {
		return filepath.Join(e.Path, "Resources", "views")
	}
	return filepath.Join(e.Path, "src", "Resources", "views")
}
