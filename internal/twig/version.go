package twig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const VersionCommentPrefix = "shopware-block:"

var (
	shopwareVersion     string
	shopwareVersionOnce sync.Once
)

func IsStorefrontTemplate(uri string) bool {
	return strings.Contains(uri, "src/Storefront/Resources/views/storefront") ||
		strings.Contains(uri, "vendor/shopware/storefront/Resources/views/storefront")
}

func DetectShopwareVersion(projectRoot string) string {
	shopwareVersionOnce.Do(func() {
		shopwareVersion = detectShopwareVersionFromComposer(projectRoot)
	})
	return shopwareVersion
}

func detectShopwareVersionFromComposer(projectRoot string) string {
	composerLockPath := filepath.Join(projectRoot, "composer.lock")

	data, err := os.ReadFile(composerLockPath)
	if err != nil {
		return "next"
	}

	var composerLock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}

	if err := json.Unmarshal(data, &composerLock); err != nil {
		return "next"
	}

	for _, pkg := range composerLock.Packages {
		if pkg.Name == "shopware/storefront" {
			return pkg.Version
		}
	}

	return "next"
}

func FindOriginalStorefrontHash(hashes []TwigBlockHash) *TwigBlockHash {
	return FindOriginalStorefrontHashForExtends(hashes, "")
}

func FindOriginalStorefrontHashForExtends(hashes []TwigBlockHash, extendsFile string) *TwigBlockHash {
	if extendsFile != "" {
		normalizedExtends := normalizeTemplatePath(extendsFile)
		for _, hash := range hashes {
			normalizedHashPath := normalizeTemplatePath(hash.RelativePath)
			if normalizedHashPath == normalizedExtends {
				return &hash
			}
		}
	}

	for _, hash := range hashes {
		if strings.HasPrefix(hash.RelativePath, "@Storefront/") ||
			strings.Contains(hash.AbsolutePath, "vendor/shopware/storefront/") ||
			strings.Contains(hash.AbsolutePath, "src/Storefront/") {
			return &hash
		}
	}
	return nil
}

// ResolveBlockLine returns the absolute path and 0-based line of blockName as
// defined in the template referenced by hash. When the block is not present in
// the parsed index (e.g. parser issues), it falls back to the file start.
func (idx *TwigIndexer) ResolveBlockLine(hash *TwigBlockHash, blockName string) (string, int) {
	files, _ := idx.GetTwigFilesByRelPath(hash.RelativePath)
	for _, f := range files {
		if f.Path == hash.AbsolutePath {
			if block, ok := f.Blocks[blockName]; ok {
				return f.Path, block.Line - 1
			}
			break
		}
	}

	return hash.AbsolutePath, 0
}

func normalizeTemplatePath(path string) string {
	path = strings.TrimPrefix(path, "@Storefront/")
	path = strings.TrimPrefix(path, "@")
	if idx := strings.Index(path, "/"); idx != -1 {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 && strings.HasSuffix(parts[0], "Storefront") {
			path = parts[1]
		}
	}
	return path
}

func FormatVersionComment(hash, version string) string {
	return fmt.Sprintf("{# %s %s@%s #}\n", VersionCommentPrefix, hash, version)
}
