package twig

import "strings"

func convertToRelativePath(twigPath string) string {
	index := strings.Index(twigPath, "Resources/views")
	if index != -1 {
		return strings.TrimPrefix(strings.TrimPrefix(twigPath[index+len("Resources/views"):], "/"), "/")
	}

	return strings.TrimPrefix(twigPath, "/")
}
