package twig

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ConvertToRelativePath(twigPath string) string {
	index := strings.Index(twigPath, "Resources/views")
	if index != -1 {
		path := strings.TrimPrefix(strings.TrimPrefix(twigPath[index:], "Resources/views"), "/")

		if path == "" {
			return ""
		}

		return fmt.Sprintf("@Storefront/%s", path)
	}

	path := strings.TrimPrefix(twigPath, "/")

	if path == "" {
		return ""
	}

	return fmt.Sprintf("@Storefront/%s", path)
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
