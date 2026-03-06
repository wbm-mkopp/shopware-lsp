package twig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
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

func ResolveOriginalStorefrontHashForBlock(indexer *TwigIndexer, blockName, extendsFile string) *TwigBlockHash {
	if indexer == nil {
		return nil
	}

	allBlockHashes, err := indexer.GetTwigBlockHashes(blockName)
	if err != nil {
		return nil
	}

	originalHash := FindOriginalStorefrontHashForExtends(allBlockHashes, extendsFile)
	if originalHash != nil || len(allBlockHashes) > 0 {
		return originalHash
	}

	return resolveOriginalHashFromExtendsFile(indexer, blockName, extendsFile)
}

func resolveOriginalHashFromExtendsFile(indexer *TwigIndexer, blockName, extendsFile string) *TwigBlockHash {
	if extendsFile == "" || indexer == nil {
		return nil
	}

	parentFiles, err := indexer.GetTwigFilesByRelPath(extendsFile)
	if err != nil || len(parentFiles) == 0 {
		return nil
	}

	for _, parentFile := range parentFiles {
		runtimeHash, runtimeErr := FindBlockHashInTemplateFile(parentFile.Path, blockName)
		if runtimeErr != nil || runtimeHash == nil {
			continue
		}
		return runtimeHash
	}

	return nil
}

func FindBlockHashInTemplateFile(filePath, blockName string) (*TwigBlockHash, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())); err != nil {
		return nil, err
	}

	tree := parser.Parse(content, nil)
	defer tree.Close()

	twigFile, err := ParseTwig(filePath, tree.RootNode(), content)
	if err != nil {
		return nil, err
	}

	block, ok := twigFile.Blocks[blockName]
	if !ok {
		return nil, nil
	}

	return &TwigBlockHash{
		Name:         block.Name,
		RelativePath: ConvertToRelativePath(filePath),
		AbsolutePath: filePath,
		Hash:         block.Hash,
		Text:         block.Text,
	}, nil
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
