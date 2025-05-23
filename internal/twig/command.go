package twig

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	tree_sitter_helper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigCommandProvider struct {
	extensionIndex *extension.ExtensionIndexer
}

func NewTwigCommandProvider(server *lsp.Server) *TwigCommandProvider {
	extensionIndex, _ := server.GetIndexer("extension.indexer")

	return &TwigCommandProvider{
		extensionIndex: extensionIndex.(*extension.ExtensionIndexer),
	}
}

func (t *TwigCommandProvider) GetCommands(ctx context.Context) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		"shopware/twig/extendBlock": t.extendBlock,
	}
}

func (t *TwigCommandProvider) extendBlock(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		TextUri   string `json:"textUri"`
		BlockName string `json:"blockName"`
		Extension string `json:"extension"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, err
	}

	extension := t.extensionIndex.GetExtensionByName(params.Extension)
	if extension == nil {
		return protocol.NewLspError("Extension not found", "extension.not_found"), nil
	}

	originalPath := strings.TrimPrefix(params.TextUri, "file://")

	// Extract Resources/views/storefront part from the original path
	resourcesIndex := strings.Index(originalPath, "Resources/views/storefront")
	if resourcesIndex == -1 {
		return protocol.NewLspError("Not a storefront view file", "view.not_storefront"), nil
	}

	storefrontRelativePath := originalPath[resourcesIndex+16:]
	extensionViewPath := path.Join(extension.GetStorefrontViewsPath(), storefrontRelativePath)
	extensionViewPathDir := path.Dir(extensionViewPath)

	if _, err := os.Stat(extensionViewPathDir); os.IsNotExist(err) {
		if err := os.MkdirAll(extensionViewPathDir, 0755); err != nil {
			log.Printf("Failed to create directory: %s", extensionViewPathDir)
			return protocol.NewLspError("Failed to create directory", "directory.create_failed"), nil
		}
	}

	_, err := os.Stat(extensionViewPath)

	if os.IsNotExist(err) {
		if err := os.WriteFile(extensionViewPath, []byte("{% sw_extends \"@Storefront/"+storefrontRelativePath+"\" %}\n"), 0644); err != nil {
			log.Printf("Failed to create file: %s", extensionViewPath)
			return protocol.NewLspError("Failed to create file", "file.create_failed"), nil
		}
	}

	parser := tree_sitter.NewParser()
	_ = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))

	currentContent, err := os.ReadFile(extensionViewPath)

	if err != nil {
		return protocol.NewLspError("Failed to read file", "file.read_failed"), nil
	}

	tree := parser.Parse(currentContent, nil)

	blocks := tree_sitter_helper.FindAll(tree.RootNode(), tree_sitter_helper.TwigBlockWithNamePattern(params.BlockName), currentContent)

	if len(blocks) > 0 {
		return protocol.NewLspError("Block already exists", "block.already_exists"), nil
	}

	currentContent = append(currentContent, []byte("\n\n{% block "+params.BlockName+" %}\n\n{% endblock %}\n")...)
	if err := os.WriteFile(extensionViewPath, currentContent, 0644); err != nil {
		log.Printf("Failed to write file: %s", extensionViewPath)
		return protocol.NewLspError("Failed to write file", "file.write_failed"), nil
	}

	tree = parser.Parse(currentContent, nil)
	blocks = tree_sitter_helper.FindAll(tree.RootNode(), tree_sitter_helper.TwigBlockWithNamePattern(params.BlockName), currentContent)

	if len(blocks) == 0 {
		return protocol.NewLspError("Block not found after creation", "block.not_found"), nil
	}

	return map[string]any{
		"uri":  "file://" + extensionViewPath,
		"line": blocks[0].StartPosition().Row + 1,
	}, nil
}
