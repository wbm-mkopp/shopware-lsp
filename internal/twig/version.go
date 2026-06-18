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

// IsStorefrontTemplate reports whether path is an upstream template source (core
// Storefront or a store.shopware.com plugin) rather than a local override.
func IsStorefrontTemplate(uri string) bool {
	return IsOriginalTemplateSource(uri)
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
		for i := range hashes {
			hash := &hashes[i]
			if twigRelPathsEquivalent(hash.RelativePath, extendsFile) {
				return hash
			}
		}

		var best *TwigBlockHash
		bestScore := 0
		for i := range hashes {
			hash := &hashes[i]
			score, ok := twigHashExtendsMatchScore(hash, extendsFile)
			if !ok {
				continue
			}
			if best == nil || score > bestScore {
				best = hash
				bestScore = score
			}
		}
		return best
	}

	return pickBestOriginalHash(hashes)
}

func twigHashExtendsMatchScore(hash *TwigBlockHash, extendsFile string) (int, bool) {
	if !IsOriginalTemplateSource(hash.AbsolutePath) {
		return 0, false
	}

	extendsBundle, extendsView := splitTwigRelPath(extendsFile)
	hashView := normalizeTemplatePath(hash.RelativePath)
	if !twigViewPathsMatch(hashView, extendsView) {
		return 0, false
	}

	if extendsBundle == "" {
		return 1, true
	}

	hashBundle, _ := splitTwigRelPath(hash.RelativePath)
	if strings.EqualFold(hashBundle, extendsBundle) {
		score := 2
		if strings.EqualFold(extendsBundle, "Storefront") && isCoreStorefrontPath(hash.AbsolutePath) {
			score = 3
		}
		return score, true
	}

	if isStoreShopwarePluginStorefrontPath(hash.AbsolutePath) {
		resolvedBundle := resolveTwigBundleNamespace(hash.AbsolutePath)
		if strings.EqualFold(resolvedBundle, extendsBundle) {
			return 2, true
		}
	}

	return 0, false
}

func pickBestOriginalHash(hashes []TwigBlockHash) *TwigBlockHash {
	var coreHash *TwigBlockHash
	var pluginHash *TwigBlockHash
	// When multiple store plugins share a block name, pick the lowest bundle
	// name so the result is stable regardless of index iteration order.
	var pluginBundle string

	for i := range hashes {
		hash := &hashes[i]
		if !IsOriginalTemplateSource(hash.AbsolutePath) {
			continue
		}

		if isCoreStorefrontPath(hash.AbsolutePath) {
			if coreHash == nil {
				coreHash = hash
			}
			continue
		}

		bundle, _ := splitTwigRelPath(hash.RelativePath)
		lowerBundle := strings.ToLower(bundle)
		if pluginHash == nil || lowerBundle < pluginBundle {
			pluginHash = hash
			pluginBundle = lowerBundle
		}
	}

	if coreHash != nil {
		return coreHash
	}

	return pluginHash
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
	if originalHash != nil {
		return originalHash
	}

	return resolveOriginalHashFromExtendsFile(indexer, blockName, extendsFile)
}

func resolveOriginalHashFromExtendsFile(indexer *TwigIndexer, blockName, extendsFile string) *TwigBlockHash {
	if extendsFile == "" || indexer == nil {
		return nil
	}

	parentFiles, err := lookupParentFilesForExtends(indexer, extendsFile)
	if err != nil || len(parentFiles) == 0 {
		return nil
	}

	for _, parentFile := range parentFiles {
		if !IsOriginalTemplateSource(parentFile.Path) {
			continue
		}
		runtimeHash, runtimeErr := FindBlockHashInTemplateFile(parentFile.Path, blockName)
		if runtimeErr != nil || runtimeHash == nil {
			continue
		}
		return runtimeHash
	}

	return nil
}

func lookupParentFilesForExtends(indexer *TwigIndexer, extendsFile string) ([]TwigFile, error) {
	parentFiles, err := indexer.GetTwigFilesByRelPath(extendsFile)
	if err != nil {
		return nil, err
	}
	if len(parentFiles) > 0 {
		return parentFiles, nil
	}

	parentFiles, err = indexer.GetTwigFilesByRelPathCaseInsensitive(extendsFile)
	if err != nil {
		return nil, err
	}
	if len(parentFiles) > 0 {
		return parentFiles, nil
	}

	return indexer.GetTwigFilesByRelPathViewMatch(extendsFile)
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
	_, view := splitTwigRelPath(path)
	return view
}

func FormatVersionComment(hash, version string) string {
	return fmt.Sprintf("{# %s %s@%s #}\n", VersionCommentPrefix, hash, version)
}
