package twig

import (
	"path"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigIndexer struct {
	twigFileIndex  *indexer.DataIndexer[TwigFile]
	twigBlockIndex *indexer.DataIndexer[TwigBlock]
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

	return &TwigIndexer{
		twigFileIndex:  twigFileIndex,
		twigBlockIndex: twigBlockIndex,
	}, nil
}

func (idx *TwigIndexer) ID() string {
	return "twig.indexer"
}

func (idx *TwigIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	if !strings.HasSuffix(path, ".twig") {
		return nil
	}

	if strings.Contains(path, "Resources/app/administration") {
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

func (idx *TwigIndexer) RemovedFiles(paths []string) error {
	if err := idx.twigFileIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	if err := idx.twigBlockIndex.BatchDeleteByFilePaths(paths); err != nil {
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

	return nil
}

func (idx *TwigIndexer) GetAllTemplateFiles() ([]string, error) {
	return idx.twigFileIndex.GetAllKeys()
}

func (idx *TwigIndexer) GetTwigFilesByRelPath(relPath string) ([]TwigFile, error) {
	return idx.twigFileIndex.GetValues(relPath)
}
