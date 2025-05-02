package snippet

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

type SnippetCommandProvider struct {
	lsp *lsp.Server
}

func NewSnippetCommandProvider(lsp *lsp.Server) *SnippetCommandProvider {
	return &SnippetCommandProvider{
		lsp: lsp,
	}
}

func (s *SnippetCommandProvider) GetCommands(ctx context.Context) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		"shopware/snippet/getPossibleSnippetFilse": s.getPossibleSnippets,
		"shopware/snippet/create":                  s.createSnippet,
	}
}

func (s *SnippetCommandProvider) getPossibleSnippets(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		FileURI string `json:"fileUri"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for getPossibleSnippets: %w", err)
	}

	// Convert URI to file path
	filePath := strings.TrimPrefix(params.FileURI, "file://")

	// // Find Resources directory
	dirPath := filepath.Dir(filePath)
	resourcesFound := false

	for {
		if filepath.Base(dirPath) == "Resources" {
			resourcesFound = true
			break
		}

		parent := filepath.Dir(dirPath)
		if parent == dirPath {
			// We've reached the root directory
			break
		}
		dirPath = parent
	}

	if !resourcesFound {
		return nil, fmt.Errorf("resources directory not found in any parent directory of %s", filePath)
	}

	snippetDir := filepath.Join(dirPath, "snippet")

	// Find possible snippets
	possibleSnippets := findPossibleSnippets(snippetDir)

	// The user didn't created one yet, we create for him one
	if len(possibleSnippets) == 0 {
		if err := os.Mkdir(filepath.Join(snippetDir, "en_GB"), os.ModePerm); err != nil {
			return nil, err
		}

		possibleSnippets = []SnippetFile{
			{
				Path:  filepath.Join(snippetDir, "en_GB", "storefront.en-GB.json"),
				Name:  "storefront.en-GB.json",
				Value: "",
			},
		}
	}

	// Return success message
	return map[string]interface{}{
		"paths": possibleSnippets,
	}, nil
}

func (s *SnippetCommandProvider) createSnippet(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		FileURI    string        `json:"fileUri"`
		SnippetKey string        `json:"snippetKey"`
		Snippets   []SnippetFile `json:"snippets"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for createSnippet: %w", err)
	}

	files := make([]string, len(params.Snippets))

	for _, snippet := range params.Snippets {
		fileContent, err := os.ReadFile(snippet.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", snippet.Path, err)
		}

		newFile, err := sjson.SetBytes(fileContent, params.SnippetKey, snippet.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to set snippet %s in file %s: %w", params.SnippetKey, snippet.Path, err)
		}

		if err := os.WriteFile(snippet.Path, pretty.Pretty(newFile), os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", snippet.Path, err)
		}

		files = append(files, snippet.Path)
	}

	if err := s.lsp.FileScanner().IndexFiles(ctx, files); err != nil {
		return nil, fmt.Errorf("failed to index files: %w", err)
	}

	s.lsp.PublishDiagnostics(ctx, []string{params.FileURI})

	return nil, nil
}

type SnippetFile struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func findPossibleSnippets(dirPath string) []SnippetFile {
	var possibleSnippets []SnippetFile

	_ = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".json" {
			possibleSnippets = append(possibleSnippets, SnippetFile{
				Path:  path,
				Name:  filepath.Base(path),
				Value: "",
			})
		}

		return nil
	})

	// Sort so that en_GB files come first
	for i := 0; i < len(possibleSnippets); i++ {
		if strings.Contains(possibleSnippets[i].Path, "en_GB") {
			// Move this item to the beginning of the slice
			possibleSnippets = append([]SnippetFile{possibleSnippets[i]}, append(possibleSnippets[:i], possibleSnippets[i+1:]...)...)
		}
	}

	return possibleSnippets
}
