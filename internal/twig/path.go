package twig

import (
	"path/filepath"
	"strings"
)

func convertToRelativePath(twigPath string) string {
	index := strings.Index(twigPath, "Resources/views")
	if index != -1 {
		return strings.TrimPrefix(strings.TrimPrefix(twigPath[index+len("Resources/views"):], "/"), "/")
	}

	return strings.TrimPrefix(twigPath, "/")
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
