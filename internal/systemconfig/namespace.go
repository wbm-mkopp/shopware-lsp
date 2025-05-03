package systemconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SystemConfigEntry represents a system config entry with namespace
type SystemConfigEntry struct {
	Namespace string
	Name      string
	Label     string
	Type      string
	Component string
	FilePath  string
	Line      uint32
}

// GetNamespaceFromPath extracts the namespace from the file path by looking for composer.json or manifest.xml
func GetNamespaceFromPath(filePath string) (string, error) {
	// Get the directory of the file
	dir := filepath.Dir(filePath)

	// Get the file name without extension
	fileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	// Look for composer.json or manifest.xml in parent directories
	for {
		composerPath := filepath.Join(dir, "composer.json")
		manifestPath := filepath.Join(dir, "manifest.xml")

		// Check if composer.json exists
		if _, err := os.Stat(composerPath); err == nil {
			// Parse composer.json to get the namespace
			namespace, err := getNamespaceFromComposerJson(composerPath, fileName)
			if err != nil {
				return "", err
			}
			return namespace, nil
		}

		// Check if manifest.xml exists
		if _, err := os.Stat(manifestPath); err == nil {
			// Parse manifest.xml to get the namespace
			namespace, err := getNamespaceFromManifestXml(manifestPath)
			if err != nil {
				return "", err
			}
			return namespace, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			// We've reached the root directory
			break
		}
		dir = parentDir
	}

	// If no composer.json or manifest.xml is found, use the file name as namespace
	return fmt.Sprintf("core.%s", fileName), nil
}

// getNamespaceFromComposerJson extracts the namespace from composer.json
func getNamespaceFromComposerJson(composerPath, fileName string) (string, error) {
	// Read composer.json
	data, err := os.ReadFile(composerPath)
	if err != nil {
		return "", err
	}

	// Parse composer.json
	var composer struct {
		Name  string `json:"name"`
		Extra struct {
			ShopwarePluginClass string `json:"shopware-plugin-class"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(data, &composer); err != nil {
		return "", err
	}

	if composer.Extra.ShopwarePluginClass == "" {
		return fmt.Sprintf("core.%s", fileName), nil
	}

	// Extract the plugin name from the shopware-plugin-class
	parts := strings.Split(composer.Extra.ShopwarePluginClass, "\\")
	pluginName := parts[len(parts)-1]

	return pluginName + ".config", nil
}

// getNamespaceFromManifestXml extracts the namespace from manifest.xml
func getNamespaceFromManifestXml(manifestPath string) (string, error) {
	// Read manifest.xml
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}

	// Parse manifest.xml using tree-sitter
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML())); err != nil {
		return "", fmt.Errorf("failed to set language: %w", err)
	}

	tree := parser.Parse(data, nil)
	defer tree.Close()

	// Find the name element
	nameNode := treesitterhelper.FindFirst(tree.RootNode(), treesitterhelper.And(
		treesitterhelper.NodeKind("element"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("STag"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("name"),
			)),
		)),
	), data)

	if nameNode == nil {
		return "", nil
	}

	// Extract the name value
	contentNode := treesitterhelper.FindFirst(nameNode, treesitterhelper.NodeKind("content"), data)
	if contentNode == nil {
		return "", nil
	}

	charDataNode := treesitterhelper.FindFirst(contentNode, treesitterhelper.NodeKind("CharData"), data)
	if charDataNode == nil {
		return "", nil
	}

	namespace := strings.TrimSpace(string(charDataNode.Utf8Text(data)))
	if namespace == "" {
		return "", nil
	}

	return namespace, nil
}

// IndexSystemConfigFile indexes a system config file and returns the entries
func IndexSystemConfigFile(data []byte, filePath string) ([]SystemConfigEntry, error) {
	// Check if it's a system config file
	if !IsSystemConfigXML(data) {
		return nil, fmt.Errorf("not a system config file")
	}

	// Get the namespace
	namespace, err := GetNamespaceFromPath(filePath)
	if err != nil {
		return nil, err
	}

	log.Printf("Namespace: %s", namespace)

	// Parse the XML
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML())); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	tree := parser.Parse(data, nil)
	defer tree.Close()

	// Find all system config fields
	fields := FindAllSystemConfigFields(tree.RootNode(), data, filePath)

	// Create entries with namespace
	entries := make([]SystemConfigEntry, 0, len(fields))
	for _, field := range fields {
		entries = append(entries, SystemConfigEntry{
			Namespace: namespace,
			Name:      field.Name,
			Label:     field.Label,
			Type:      field.Type,
			Component: field.Component,
			FilePath:  filePath,
			Line:      field.Line,
		})
	}

	return entries, nil
}
