package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shopware/shopware-lsp/internal/snippet"
	"path/filepath"
	"slices"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

type SnippetCommandProvider struct {
	snippetIndexer *snippet.SnippetIndexer
	host           EditHost
}

func NewSnippetCommandProvider(snippetIndexer *snippet.SnippetIndexer, host EditHost) *SnippetCommandProvider {
	return &SnippetCommandProvider{snippetIndexer: snippetIndexer, host: host}
}

func (s *SnippetCommandProvider) GetCommands(ctx context.Context) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		"shopware/snippet/storefront/getPossibleSnippetFiles": s.getPossibleSnippets,
		"shopware/snippet/storefront/create":                  s.createSnippet,
		"shopware/snippet/storefront/all":                     s.allSnippets,
		"shopware/snippet/admin/all":                          s.allAdminSnippets,
		"shopware/snippet/admin/getPossibleSnippetFiles":      s.getPossibleAdminSnippets,
		"shopware/snippet/admin/create":                       s.createAdminSnippet,
	}
}

func (s *SnippetCommandProvider) allSnippets(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	type snippetItem struct {
		Key  string `json:"key"`
		Text string `json:"text"`
		File string `json:"file"`
	}

	var allSnippets = make(map[string]snippetItem)

	storefrontSnippets, err := s.snippetIndexer.GetAllFrontendSnippets()
	if err != nil {
		return nil, fmt.Errorf("failed to get storefront snippets: %w", err)
	}

	for _, snippet := range storefrontSnippets {
		if _, ok := allSnippets[snippet.Key]; !ok {
			allSnippets[snippet.Key] = snippetItem{
				Key:  snippet.Key,
				Text: snippet.Text,
				File: snippet.File,
			}
		}

		fileName := filepath.Base(snippet.File)
		if strings.Contains(fileName, "en_GB") {
			// Prefer en_GB snippets
			allSnippets[snippet.Key] = snippetItem{
				Key:  snippet.Key,
				Text: snippet.Text,
				File: snippet.File,
			}
		}
	}

	var allSnippetsList []snippetItem
	for _, snippet := range allSnippets {
		allSnippetsList = append(allSnippetsList, snippet)
	}

	slices.SortFunc(allSnippetsList, func(a, b snippetItem) int {
		return strings.Compare(a.Key, b.Key)
	})

	return allSnippetsList, nil
}

func (s *SnippetCommandProvider) allAdminSnippets(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	type snippetItem struct {
		Key  string `json:"key"`
		Text string `json:"text"`
		File string `json:"file"`
	}

	var allSnippets = make(map[string]snippetItem)

	adminSnippets, err := s.snippetIndexer.GetAllAdminSnippets()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin snippets: %w", err)
	}

	for _, snippet := range adminSnippets {
		if _, ok := allSnippets[snippet.Key]; !ok {
			allSnippets[snippet.Key] = snippetItem{
				Key:  snippet.Key,
				Text: snippet.Text,
				File: snippet.File,
			}
		}

		fileName := filepath.Base(snippet.File)
		// Prefer en-GB snippets
		if strings.Contains(fileName, "en-GB") || strings.Contains(fileName, "en.json") {
			allSnippets[snippet.Key] = snippetItem{
				Key:  snippet.Key,
				Text: snippet.Text,
				File: snippet.File,
			}
		}
	}

	var allSnippetsList []snippetItem
	for _, snippet := range allSnippets {
		allSnippetsList = append(allSnippetsList, snippet)
	}

	slices.SortFunc(allSnippetsList, func(a, b snippetItem) int {
		return strings.Compare(a.Key, b.Key)
	})

	return allSnippetsList, nil
}

func (s *SnippetCommandProvider) getPossibleSnippets(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		FileURI string `json:"fileUri"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for getPossibleSnippets: %w", err)
	}

	// Convert URI to file path
	filePath, err := uriutil.Path(params.FileURI)
	if err != nil {
		return nil, fmt.Errorf("resolve document URI: %w", err)
	}

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
	possibleSnippets, err := s.possibleSnippets(ctx, snippetDir, false)

	if err != nil {
		return nil, err
	}
	// Suggest a path without creating it.
	if len(possibleSnippets) == 0 {

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

	return s.createSnippets(ctx, params.SnippetKey, params.Snippets)
}

func (s *SnippetCommandProvider) createSnippets(ctx context.Context, key string, snippets []SnippetFile) (interface{}, error) {
	plan := rewrite.WorkspacePlan{}
	for _, snippet := range snippets {
		uri := uriutil.FileURI(snippet.Path)
		snapshot, err := targetSnapshot(ctx, s.host, uri)
		if err != nil {
			return nil, err
		}
		source := "{}"
		if snapshot.Document != nil {
			source = snapshot.Document.SourceString()
		}
		content, err := sjson.Set(source, key, snippet.Value)
		if err != nil {
			return nil, fmt.Errorf("set snippet %s: %w", key, err)
		}
		if err := addReplacement(&plan, uri, snapshot, string(pretty.Pretty([]byte(content)))); err != nil {
			return nil, err
		}
	}
	edit, err := s.host.WorkspaceEdit(ctx, plan)
	if err != nil {
		return nil, err
	}
	return EditResponse{Edit: edit}, nil
}

type SnippetFile struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s *SnippetCommandProvider) getPossibleAdminSnippets(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		FileURI string `json:"fileUri"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for getPossibleAdminSnippets: %w", err)
	}

	// Convert URI to file path
	filePath, err := uriutil.Path(params.FileURI)
	if err != nil {
		return nil, fmt.Errorf("resolve document URI: %w", err)
	}

	// Find Resources/app/administration directory
	dirPath := filepath.Dir(filePath)
	administrationFound := false
	var resourcesDir string

	for {
		if filepath.Base(dirPath) == "administration" {
			parent := filepath.Dir(dirPath)
			if filepath.Base(parent) == "app" {
				grandparent := filepath.Dir(parent)
				if filepath.Base(grandparent) == "Resources" {
					administrationFound = true
					resourcesDir = grandparent
					break
				}
			}
		}

		parent := filepath.Dir(dirPath)
		if parent == dirPath {
			// We've reached the root directory
			break
		}
		dirPath = parent
	}

	if !administrationFound {
		return nil, fmt.Errorf("resources/app/administration directory not found in any parent directory of %s", filePath)
	}

	// Search entire administration/src directory for snippet files
	administrationSrcDir := filepath.Join(resourcesDir, "app", "administration", "src")

	// Find possible snippets anywhere under administration/src in snippet/ directories
	possibleSnippets, err := s.possibleSnippets(ctx, administrationSrcDir, true)

	if err != nil {
		return nil, err
	}
	// Suggest a path without creating it.
	if len(possibleSnippets) == 0 {
		snippetDir := filepath.Join(administrationSrcDir, "snippet")

		possibleSnippets = []SnippetFile{
			{
				Path:  filepath.Join(snippetDir, "en-GB.json"),
				Name:  "en-GB.json",
				Value: "",
			},
		}
	}

	// Return success message
	return map[string]interface{}{
		"paths": possibleSnippets,
	}, nil
}

func (s *SnippetCommandProvider) createAdminSnippet(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	var params struct {
		FileURI    string        `json:"fileUri"`
		SnippetKey string        `json:"snippetKey"`
		Snippets   []SnippetFile `json:"snippets"`
	}

	if err := json.Unmarshal(*args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments for createAdminSnippet: %w", err)
	}

	return s.createSnippets(ctx, params.SnippetKey, params.Snippets)
}

func (s *SnippetCommandProvider) possibleSnippets(ctx context.Context, directory string, admin bool) ([]SnippetFile, error) {
	paths, err := s.host.ResourcePaths(ctx, directory, 10000)
	if err != nil {
		return nil, err
	}
	var result []SnippetFile
	for _, path := range paths {
		if filepath.Ext(path) != ".json" || (admin && filepath.Base(filepath.Dir(path)) != "snippet") {
			continue
		}
		result = append(result, SnippetFile{Path: path, Name: filepath.Base(path)})
	}
	slices.SortFunc(result, func(a, b SnippetFile) int {
		preferred := func(file SnippetFile) bool {
			return strings.Contains(file.Path, "en_GB") || strings.Contains(file.Name, "en-GB") || file.Name == "en.json"
		}
		if preferred(a) != preferred(b) {
			if preferred(a) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	return result, nil
}
