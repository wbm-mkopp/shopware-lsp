package extension

import (
	"log"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/php"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type ExtensionIndexer struct {
	indexer *indexer.DataIndexer[ShopwareExtension]
}

func NewExtensionIndexer(configDir string) (*ExtensionIndexer, error) {
	indexer, err := indexer.NewDataIndexer[ShopwareExtension](filepath.Join(configDir, "extension.db"))
	if err != nil {
		return nil, err
	}

	return &ExtensionIndexer{
		indexer: indexer,
	}, nil
}

func (idx *ExtensionIndexer) ID() string {
	return "extension.indexer"
}

func (idx *ExtensionIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	if !isValidForIndex(path) {
		return nil
	}

	switch filepath.Ext(path) {
	case ".php":
		return idx.indexBundle(path, node, fileContent)
	case ".xml":
		return idx.indexApp(path, node, fileContent)
	default:
		return nil
	}
}

func (idx *ExtensionIndexer) indexBundle(path string, node *tree_sitter.Node, fileContent []byte) error {
	classes := php.GetClassesOfFileWithParser(path, node, fileContent)
	if len(classes) == 0 {
		return nil
	}
	for _, class := range classes {
		if isShopwareBundle(class) {
			extension := createBundleFromClass(class)
			log.Printf("Indexing bundle: %s in %s", extension.Name, path)
			return idx.indexer.SaveItem(path, extension.Name, extension)
		}
	}
	return nil
}

func (idx *ExtensionIndexer) indexApp(path string, node *tree_sitter.Node, fileContent []byte) error {
	if filepath.Base(path) != "manifest.xml" {
		return nil
	}

	manifest, err := ParseManifestXml(path, node, fileContent)

	if err != nil {
		log.Printf("Error parsing manifest.xml: %v", err)
		return err
	}

	if manifest == nil {
		return nil
	}

	app := ShopwareExtension{
		Name: manifest.Name,
		Type: ShopwareExtensionTypeApp,
		Path: filepath.Dir(path),
	}

	log.Printf("Indexing app: %s in %s", app.Name, path)

	return idx.indexer.SaveItem(path, manifest.Name, app)
}

func (idx *ExtensionIndexer) RemovedFiles(paths []string) error {
	return idx.indexer.BatchDeleteByFilePaths(paths)
}

func (idx *ExtensionIndexer) Close() error {
	return idx.indexer.Close()
}

func (idx *ExtensionIndexer) Clear() error {
	return idx.indexer.Clear()
}

func (idx *ExtensionIndexer) GetAll() ([]ShopwareExtension, error) {
	return idx.indexer.GetAllValues()
}
