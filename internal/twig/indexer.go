package twig

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigIndexer struct {
	twigFileIndex     *indexer.DataIndexer[TwigFile]
	twigBlockIndex    *indexer.DataIndexer[TwigBlock]
	twigFunctionIndex *indexer.DataIndexer[TwigFunction]
	twigFilterIndex   *indexer.DataIndexer[TwigFilter]
}

func NewTwigIndexer(configDir string) (*TwigIndexer, error) {
	twigFileIndex, err := indexer.NewDataIndexer[TwigFile](path.Join(configDir, "twig_file.index"))

	if err != nil {
		return nil, err
	}

	twigBlockIndex, err := indexer.NewDataIndexer[TwigBlock](path.Join(configDir, "twig_block.index"))

	if err != nil {
		return nil, err
	}

	twigFunctionIndex, err := indexer.NewDataIndexer[TwigFunction](path.Join(configDir, "twig_function.index"))
	if err != nil {
		return nil, err
	}

	twigFilterIndex, err := indexer.NewDataIndexer[TwigFilter](path.Join(configDir, "twig_filter.index"))
	if err != nil {
		return nil, err
	}

	return &TwigIndexer{
		twigFileIndex:     twigFileIndex,
		twigBlockIndex:    twigBlockIndex,
		twigFunctionIndex: twigFunctionIndex,
		twigFilterIndex:   twigFilterIndex,
	}, nil
}

func (idx *TwigIndexer) ID() string {
	return "twig.indexer"
}

func (idx *TwigIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	switch filepath.Ext(path) {
	case ".twig":
		return idx.indexTwig(path, node, fileContent)
	case ".php":
		return idx.indexExtension(path, node, fileContent)
	default:
		return nil
	}
}

func (idx *TwigIndexer) indexTwig(path string, node *tree_sitter.Node, fileContent []byte) error {
	if strings.Contains(path, "Resources/app/administration") || strings.Contains(path, "Migration/Fixtures") || strings.Contains(path, ".phpdoc/template") {
		return nil
	}

	file, err := ParseTwig(path, node, fileContent)
	if err != nil {
		return err
	}

	if err := idx.twigFileIndex.SaveItem(path, file.RelPath, *file); err != nil {
		return err
	}

	twigBlocks := make(map[string]map[string]TwigBlock)
	twigBlocks[file.Path] = make(map[string]TwigBlock)

	for _, block := range file.Blocks {
		if _, ok := twigBlocks[file.Path][block.Name]; !ok {
			twigBlocks[file.Path][block.Name] = block
		}
	}

	if err := idx.twigBlockIndex.BatchSaveItems(twigBlocks); err != nil {
		return err
	}

	return nil
}

func (idx *TwigIndexer) indexExtension(path string, node *tree_sitter.Node, fileContent []byte) error {
	functions, filters, err := ParseTwigExtension(path, node, fileContent)
	if err != nil {
		return err
	}

	if len(functions) == 0 && len(filters) == 0 {
		return nil
	}

	functionsMap := make(map[string]map[string]TwigFunction)
	filtersMap := make(map[string]map[string]TwigFilter)

	for _, function := range functions {
		if _, ok := functionsMap[function.FilePath]; !ok {
			functionsMap[function.FilePath] = make(map[string]TwigFunction)
		}
		functionsMap[function.FilePath][function.Name] = function
	}

	for _, filter := range filters {
		if _, ok := filtersMap[filter.FilePath]; !ok {
			filtersMap[filter.FilePath] = make(map[string]TwigFilter)
		}
		filtersMap[filter.FilePath][filter.Name] = filter
	}

	if err := idx.twigFunctionIndex.BatchSaveItems(functionsMap); err != nil {
		return err
	}

	if err := idx.twigFilterIndex.BatchSaveItems(filtersMap); err != nil {
		return err
	}

	return nil
}

func (idx *TwigIndexer) RemovedFiles(paths []string) error {
	if err := idx.twigFileIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	if err := idx.twigBlockIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	if err := idx.twigFunctionIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	if err := idx.twigFilterIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	return nil
}

func (idx *TwigIndexer) Close() error {
	if err := idx.twigBlockIndex.Close(); err != nil {
		return err
	}

	if err := idx.twigFileIndex.Close(); err != nil {
		return err
	}

	if err := idx.twigFunctionIndex.Close(); err != nil {
		return err
	}

	if err := idx.twigFilterIndex.Close(); err != nil {
		return err
	}

	return nil
}

func (idx *TwigIndexer) Clear() error {
	if err := idx.twigBlockIndex.Clear(); err != nil {
		return err
	}

	if err := idx.twigFileIndex.Clear(); err != nil {
		return err
	}

	if err := idx.twigFunctionIndex.Clear(); err != nil {
		return err
	}

	if err := idx.twigFilterIndex.Clear(); err != nil {
		return err
	}

	return nil
}

func (idx *TwigIndexer) GetAllTemplateFiles() ([]string, error) {
	return idx.twigFileIndex.GetAllKeys()
}

func (idx *TwigIndexer) GetAllTwigFunctions() ([]TwigFunction, error) {
	return idx.twigFunctionIndex.GetAllValues()
}

func (idx *TwigIndexer) GetTwigFunction(name string) ([]TwigFunction, error) {
	return idx.twigFunctionIndex.GetValues(name)
}

func (idx *TwigIndexer) GetTwigFilter(name string) ([]TwigFilter, error) {
	return idx.twigFilterIndex.GetValues(name)
}

func (idx *TwigIndexer) GetAllTwigFilters() ([]TwigFilter, error) {
	return idx.twigFilterIndex.GetAllValues()
}

func (idx *TwigIndexer) GetTwigFilesByRelPath(relPath string) ([]TwigFile, error) {
	return idx.twigFileIndex.GetValues(relPath)
}
