package admin

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// JavaScript patterns for Shopware.Component.register/extend calls
var (
	// Pattern to match Component.register/extend call expressions
	// Supports both:
	// - Shopware.Component.register(...)
	// - Component.register(...) (when destructured from Shopware)
	JSComponentCallPattern = treesitterhelper.And(
		treesitterhelper.NodeKind("call_expression"),
		treesitterhelper.HasChild(
			treesitterhelper.And(
				treesitterhelper.NodeKind("member_expression"),
				treesitterhelper.Or(
					// Full path: Shopware.Component.register / Shopware.Component.extend
					treesitterhelper.NodeText("Shopware.Component.register"),
					treesitterhelper.NodeText("Shopware.Component.extend"),
					// Destructured: Component.register / Component.extend
					treesitterhelper.NodeText("Component.register"),
					treesitterhelper.NodeText("Component.extend"),
				),
			),
		),
	)

	// Pattern to match export default { ... } statements (Vue component definitions)
	JSExportDefaultPattern = treesitterhelper.And(
		treesitterhelper.NodeKind("export_statement"),
		treesitterhelper.HasChild(treesitterhelper.NodeKind("default")),
		treesitterhelper.HasChild(treesitterhelper.NodeKind("object")),
	)

	// Pattern to match export default Shopware.Component.wrapComponentConfig({...})
	// Used for Meteor component library wrappers
	JSWrapComponentConfigPattern = treesitterhelper.And(
		treesitterhelper.NodeKind("export_statement"),
		treesitterhelper.HasChild(treesitterhelper.NodeKind("default")),
		treesitterhelper.HasChild(
			treesitterhelper.And(
				treesitterhelper.NodeKind("call_expression"),
				treesitterhelper.HasChild(
					treesitterhelper.And(
						treesitterhelper.NodeKind("member_expression"),
						treesitterhelper.Or(
							treesitterhelper.NodeText("Shopware.Component.wrapComponentConfig"),
							treesitterhelper.NodeText("Component.wrapComponentConfig"),
						),
					),
				),
			),
		),
	)
)

type AdminComponentIndexer struct {
	componentIndex  *indexer.DataIndexer[VueComponent]
	definitionIndex *indexer.DataIndexer[ComponentDefinition]
}

func NewAdminComponentIndexer(configDir string) (*AdminComponentIndexer, error) {
	componentIndex, err := indexer.NewDataIndexer[VueComponent](path.Join(configDir, "admin_component.db"))
	if err != nil {
		return nil, err
	}

	definitionIndex, err := indexer.NewDataIndexer[ComponentDefinition](path.Join(configDir, "admin_component_definition.db"))
	if err != nil {
		return nil, err
	}

	return &AdminComponentIndexer{
		componentIndex:  componentIndex,
		definitionIndex: definitionIndex,
	}, nil
}

func (idx *AdminComponentIndexer) ID() string {
	return "admin.component.indexer"
}

func (idx *AdminComponentIndexer) Index(filePath string, node *tree_sitter.Node, fileContent []byte) error {
	ext := filepath.Ext(filePath)
	if ext != ".js" && ext != ".ts" {
		return nil
	}

	// Only index files in Administration directory
	if !strings.Contains(filePath, "Resources/app/administration") {
		return nil
	}

	// Try to parse component registrations (Shopware.Component.register/extend or Component.register/extend)
	if err := idx.indexRegistrations(filePath, node, fileContent); err != nil {
		return err
	}

	// Try to parse wrapped component configs (export default Shopware.Component.wrapComponentConfig({...}))
	// Returns true if this file was a wrapComponentConfig file
	handledByWrap, err := idx.indexWrappedComponents(filePath, node, fileContent)
	if err != nil {
		return err
	}

	// Try to parse component definitions (export default { ... })
	// Skip if already handled by wrapComponentConfig to avoid duplicate indexing
	if !handledByWrap {
		if err := idx.indexDefinition(filePath, node, fileContent); err != nil {
			return err
		}
	}

	return nil
}

// indexRegistrations indexes Shopware.Component.register/extend calls
func (idx *AdminComponentIndexer) indexRegistrations(filePath string, node *tree_sitter.Node, fileContent []byte) error {
	components := parseComponentRegistrations(node, fileContent, filePath)
	if len(components) == 0 {
		return nil
	}

	batchSave := make(map[string]map[string]VueComponent)
	batchSaveDefs := make(map[string]map[string]ComponentDefinition)

	for _, comp := range components {
		if _, ok := batchSave[comp.FilePath]; !ok {
			batchSave[comp.FilePath] = make(map[string]VueComponent)
		}
		batchSave[comp.FilePath][comp.Name] = comp

		// If there's an inline definition, also save it
		if comp.InlineDefinition != nil {
			if _, ok := batchSaveDefs[comp.FilePath]; !ok {
				batchSaveDefs[comp.FilePath] = make(map[string]ComponentDefinition)
			}
			// Use component name as key for inline definitions
			batchSaveDefs[comp.FilePath][comp.Name] = *comp.InlineDefinition
		}
	}

	if err := idx.componentIndex.BatchSaveItems(batchSave); err != nil {
		return err
	}

	if len(batchSaveDefs) > 0 {
		if err := idx.definitionIndex.BatchSaveItems(batchSaveDefs); err != nil {
			return err
		}
	}

	return nil
}

// indexWrappedComponents indexes Shopware.Component.wrapComponentConfig() calls
// These are used for wrapping Meteor component library components
// Returns true if the file was handled (contains wrapComponentConfig), false otherwise
func (idx *AdminComponentIndexer) indexWrappedComponents(filePath string, node *tree_sitter.Node, fileContent []byte) (bool, error) {
	// Check if this file has an export default with wrapComponentConfig
	exportNode := treesitterhelper.FindFirst(node, JSWrapComponentConfigPattern, fileContent)
	if exportNode == nil {
		return false, nil
	}

	// Derive component name from directory name
	// e.g., /path/to/mt-card/index.ts -> "mt-card"
	componentName := deriveComponentNameFromPath(filePath)
	if componentName == "" {
		return true, nil // Still handled, just can't derive name
	}

	// Find the call expression with the config object
	callExpr := treesitterhelper.GetFirstNodeOfKind(exportNode, "call_expression")
	if callExpr == nil {
		return true, nil
	}

	// Find the arguments (the config object)
	argsNode := treesitterhelper.GetFirstNodeOfKind(callExpr, "arguments")
	if argsNode == nil {
		return true, nil
	}

	// Find the object inside the arguments
	var configObject *tree_sitter.Node
	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child.Kind() == "object" {
			configObject = child
			break
		}
	}

	if configObject == nil {
		return true, nil
	}

	// Parse the component definition from the config object
	def := parseInlineDefinition(configObject, fileContent, filePath)

	// Find template import from the root node and parse slots/blocks
	if def != nil {
		templatePath := findTemplateImport(node, fileContent)
		if templatePath != "" {
			templateAbsPath := ResolveTemplatePath(filePath, templatePath)
			def.TemplatePath = templateAbsPath // Store absolute path
			if result, err := ParseTemplateFromFile(templateAbsPath); err == nil {
				def.Slots = result.Slots
				def.Blocks = result.Blocks
			}
		}
	}

	// Create the component entry
	comp := VueComponent{
		Name:             componentName,
		FilePath:         filePath,
		Line:             int(exportNode.StartPosition().Row) + 1,
		DefinitionPath:   filePath,
		InlineDefinition: def,
	}

	// Copy props/emits/methods/computed/slots/blocks from definition to component
	if def != nil {
		comp.Props = def.Props
		comp.Emits = def.Emits
		comp.Methods = def.Methods
		comp.Computed = def.Computed
		comp.Slots = def.Slots
		comp.Blocks = def.Blocks
	}

	// Save the component
	batchSave := make(map[string]map[string]VueComponent)
	batchSave[filePath] = map[string]VueComponent{
		componentName: comp,
	}

	if err := idx.componentIndex.BatchSaveItems(batchSave); err != nil {
		return true, err
	}

	// Also save the definition
	if def != nil {
		batchSaveDefs := make(map[string]map[string]ComponentDefinition)
		batchSaveDefs[filePath] = map[string]ComponentDefinition{
			componentName: *def,
		}
		if err := idx.definitionIndex.BatchSaveItems(batchSaveDefs); err != nil {
			return true, err
		}
	}

	return true, nil
}

// deriveComponentNameFromPath extracts the component name from the file path
// e.g., /path/to/mt-card/index.ts -> "mt-card"
// e.g., /path/to/sw-button.js -> "sw-button"
func deriveComponentNameFromPath(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	// If file is index.js or index.ts, use directory name
	if base == "index.js" || base == "index.ts" {
		return filepath.Base(dir)
	}

	// Otherwise use file name without extension
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// indexDefinition indexes component definition files (export default { ... })
func (idx *AdminComponentIndexer) indexDefinition(filePath string, node *tree_sitter.Node, fileContent []byte) error {
	// Check if this file has an export default with an object
	exportNode := treesitterhelper.FindFirst(node, JSExportDefaultPattern, fileContent)
	if exportNode == nil {
		return nil
	}

	// Parse the component definition
	def := ParseComponentDefinition(node, fileContent)
	if def == nil {
		return nil
	}

	// Set the file path
	def.FilePath = filePath

	// Parse slots and blocks from the template if available
	if def.TemplatePath != "" {
		// If TemplatePath is relative, resolve it
		templateAbsPath := def.TemplatePath
		if !filepath.IsAbs(templateAbsPath) {
			templateAbsPath = ResolveTemplatePath(filePath, def.TemplatePath)
			def.TemplatePath = templateAbsPath // Store absolute path
		}
		if result, err := ParseTemplateFromFile(templateAbsPath); err == nil {
			def.Slots = result.Slots
			def.Blocks = result.Blocks
		}
	}

	// Store the definition indexed by the file path (normalized)
	// We'll use the file path as the key so we can look it up later
	normalizedPath := normalizeDefinitionPath(filePath)

	batchSave := make(map[string]map[string]ComponentDefinition)
	batchSave[filePath] = map[string]ComponentDefinition{
		normalizedPath: *def,
	}

	return idx.definitionIndex.BatchSaveItems(batchSave)
}

// normalizeDefinitionPath creates a normalized key from a definition file path
// This removes the .js/.ts extension and handles index.js files
func normalizeDefinitionPath(filePath string) string {
	// Remove extension
	ext := filepath.Ext(filePath)
	normalized := strings.TrimSuffix(filePath, ext)

	// If it ends with /index, keep the directory path
	normalized = strings.TrimSuffix(normalized, "/index")

	return normalized
}

func (idx *AdminComponentIndexer) RemovedFiles(paths []string) error {
	if err := idx.componentIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}
	return idx.definitionIndex.BatchDeleteByFilePaths(paths)
}

func (idx *AdminComponentIndexer) Close() error {
	if err := idx.componentIndex.Close(); err != nil {
		return err
	}
	return idx.definitionIndex.Close()
}

func (idx *AdminComponentIndexer) Clear() error {
	if err := idx.componentIndex.Clear(); err != nil {
		return err
	}
	return idx.definitionIndex.Clear()
}

// GetAllComponents returns all registered Vue components
func (idx *AdminComponentIndexer) GetAllComponents() ([]VueComponent, error) {
	return idx.componentIndex.GetAllValues()
}

// GetAllComponentNames returns all registered component names
func (idx *AdminComponentIndexer) GetAllComponentNames() ([]string, error) {
	return idx.componentIndex.GetAllKeys()
}

// GetComponent returns components by name (may have multiple if extended)
func (idx *AdminComponentIndexer) GetComponent(name string) ([]VueComponent, error) {
	return idx.componentIndex.GetValues(name)
}

// GetComponentDefinition returns the component definition for a given definition path
func (idx *AdminComponentIndexer) GetComponentDefinition(definitionPath string) (*ComponentDefinition, error) {
	normalizedPath := normalizeDefinitionPath(definitionPath)
	defs, err := idx.definitionIndex.GetValues(normalizedPath)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return nil, nil
	}
	return &defs[0], nil
}

// GetComponentDefinitionByName returns the inline component definition by component name
func (idx *AdminComponentIndexer) GetComponentDefinitionByName(name string) (*ComponentDefinition, error) {
	defs, err := idx.definitionIndex.GetValues(name)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return nil, nil
	}
	return &defs[0], nil
}

// GetComponentWithDefinition returns a component with its definition populated
// Multiple registrations of the same component are merged into one, preferring
// the entry with more complete data (has props, inline definition, etc.)
func (idx *AdminComponentIndexer) GetComponentWithDefinition(name string) ([]VueComponent, error) {
	components, err := idx.componentIndex.GetValues(name)
	if err != nil {
		return nil, err
	}

	if len(components) == 0 {
		return components, nil
	}

	// Populate definitions for all components
	for i := range components {
		// First try to get definition by path (for dynamic imports)
		if components[i].DefinitionPath != "" {
			def, err := idx.GetComponentDefinition(components[i].DefinitionPath)
			if err == nil && def != nil {
				components[i].Props = def.Props
				components[i].Emits = def.Emits
				components[i].Methods = def.Methods
				components[i].Computed = def.Computed
				components[i].Slots = def.Slots
				components[i].Blocks = def.Blocks
				components[i].TemplatePath = def.TemplatePath
				continue
			}
		}

		// Then try by component name (for inline definitions)
		def, err := idx.GetComponentDefinitionByName(components[i].Name)
		if err == nil && def != nil {
			components[i].Props = def.Props
			components[i].Emits = def.Emits
			components[i].Methods = def.Methods
			components[i].Computed = def.Computed
			components[i].Slots = def.Slots
			components[i].Blocks = def.Blocks
			components[i].TemplatePath = def.TemplatePath
		}
	}

	// Deduplicate: merge multiple registrations into one
	// Prefer the component with more complete data
	return deduplicateComponents(components), nil
}

// deduplicateComponents merges multiple component entries with the same name
// into a single entry, preferring entries with more complete data
func deduplicateComponents(components []VueComponent) []VueComponent {
	if len(components) <= 1 {
		return components
	}

	// Find the best component (one with the most complete data)
	best := components[0]
	for i := 1; i < len(components); i++ {
		comp := components[i]
		// Prefer component with props defined
		if len(comp.Props) > len(best.Props) {
			best = mergeComponents(best, comp)
		} else if len(comp.Props) < len(best.Props) {
			best = mergeComponents(comp, best)
		} else {
			// Same number of props, prefer one with definition path
			if comp.DefinitionPath != "" && best.DefinitionPath == "" {
				best = mergeComponents(best, comp)
			} else {
				best = mergeComponents(comp, best)
			}
		}
	}

	return []VueComponent{best}
}

// mergeComponents merges two components, taking data from 'preferred' when available,
// falling back to 'fallback' for missing data
func mergeComponents(fallback, preferred VueComponent) VueComponent {
	result := preferred

	// Use fallback values for empty fields
	if result.ExtendsComponent == "" && fallback.ExtendsComponent != "" {
		result.ExtendsComponent = fallback.ExtendsComponent
	}
	if result.ImportPath == "" && fallback.ImportPath != "" {
		result.ImportPath = fallback.ImportPath
	}
	if result.DefinitionPath == "" && fallback.DefinitionPath != "" {
		result.DefinitionPath = fallback.DefinitionPath
	}
	if len(result.Props) == 0 && len(fallback.Props) > 0 {
		result.Props = fallback.Props
	}
	if len(result.Emits) == 0 && len(fallback.Emits) > 0 {
		result.Emits = fallback.Emits
	}
	if len(result.Methods) == 0 && len(fallback.Methods) > 0 {
		result.Methods = fallback.Methods
	}
	if len(result.Computed) == 0 && len(fallback.Computed) > 0 {
		result.Computed = fallback.Computed
	}
	if len(result.Slots) == 0 && len(fallback.Slots) > 0 {
		result.Slots = fallback.Slots
	}
	if len(result.Blocks) == 0 && len(fallback.Blocks) > 0 {
		result.Blocks = fallback.Blocks
	}
	if result.TemplatePath == "" && fallback.TemplatePath != "" {
		result.TemplatePath = fallback.TemplatePath
	}

	return result
}

// parseComponentRegistrations extracts Shopware.Component.register and extend calls
func parseComponentRegistrations(root *tree_sitter.Node, content []byte, filePath string) []VueComponent {
	// Find all call expressions that match our pattern
	callNodes := treesitterhelper.FindAll(root, JSComponentCallPattern, content)

	var components []VueComponent
	for _, node := range callNodes {
		comp := parseComponentCall(node, content, filePath)
		if comp != nil {
			components = append(components, *comp)
		}
	}

	return components
}

// parseComponentCall parses a single component registration call
func parseComponentCall(node *tree_sitter.Node, content []byte, filePath string) *VueComponent {
	// Get the member_expression to determine if it's register or extend
	memberExpr := treesitterhelper.GetFirstNodeOfKind(node, "member_expression")
	if memberExpr == nil {
		return nil
	}

	memberText := string(memberExpr.Utf8Text(content))

	// Check for register or extend (both full path and destructured)
	isRegister := memberText == "Shopware.Component.register" || memberText == "Component.register"
	isExtend := memberText == "Shopware.Component.extend" || memberText == "Component.extend"

	if !isRegister && !isExtend {
		return nil
	}

	// Get arguments node
	argsNode := treesitterhelper.GetFirstNodeOfKind(node, "arguments")
	if argsNode == nil {
		return nil
	}

	comp := &VueComponent{
		FilePath: filePath,
		Line:     int(node.Range().StartPoint.Row) + 1,
	}

	// Parse arguments based on call type
	if isRegister {
		parseRegisterArgs(argsNode, content, filePath, comp)
	} else if isExtend {
		parseExtendArgs(argsNode, content, filePath, comp)
	}

	if comp.Name == "" {
		return nil
	}

	return comp
}

// parseRegisterArgs parses arguments for Component.register('name', definition | () => import('path'))
func parseRegisterArgs(argsNode *tree_sitter.Node, content []byte, filePath string, comp *VueComponent) {
	args := getArguments(argsNode)

	if len(args) < 1 {
		return
	}

	// First argument: component name (string)
	if args[0].Kind() == "string" {
		comp.Name = extractStringContent(args[0], content)
	}

	if len(args) < 2 {
		return
	}

	// Second argument: either an object (inline definition) or arrow function (dynamic import)
	secondArg := args[1]

	switch secondArg.Kind() {
	case "object":
		// Inline definition: Component.register('name', { ... })
		def := parseInlineDefinition(secondArg, content, filePath)
		comp.InlineDefinition = def
		comp.DefinitionPath = filePath // Definition is in the same file

	case "arrow_function":
		// Dynamic import: Component.register('name', () => import('path'))
		importPath := extractImportPath(secondArg, content)
		if importPath != "" {
			comp.ImportPath = importPath
			comp.DefinitionPath = resolveImportPath(filePath, importPath)
		}
	}
}

// parseExtendArgs parses arguments for Component.extend('name', 'parent', definition | () => import('path'))
func parseExtendArgs(argsNode *tree_sitter.Node, content []byte, filePath string, comp *VueComponent) {
	args := getArguments(argsNode)

	if len(args) < 2 {
		return
	}

	// First argument: component name (string)
	if args[0].Kind() == "string" {
		comp.Name = extractStringContent(args[0], content)
	}

	// Second argument: parent component name (string)
	if args[1].Kind() == "string" {
		comp.ExtendsComponent = extractStringContent(args[1], content)
	}

	if len(args) < 3 {
		return
	}

	// Third argument: either an object (inline definition) or arrow function (dynamic import)
	thirdArg := args[2]

	switch thirdArg.Kind() {
	case "object":
		// Inline definition: Component.extend('name', 'parent', { ... })
		def := parseInlineDefinition(thirdArg, content, filePath)
		comp.InlineDefinition = def
		comp.DefinitionPath = filePath

	case "arrow_function":
		// Dynamic import: Component.extend('name', 'parent', () => import('path'))
		importPath := extractImportPath(thirdArg, content)
		if importPath != "" {
			comp.ImportPath = importPath
			comp.DefinitionPath = resolveImportPath(filePath, importPath)
		}
	}
}

// getArguments returns the direct argument nodes from an arguments node
func getArguments(argsNode *tree_sitter.Node) []*tree_sitter.Node {
	var args []*tree_sitter.Node

	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		kind := child.Kind()
		// Skip punctuation
		if kind == "(" || kind == ")" || kind == "," {
			continue
		}
		args = append(args, child)
	}

	return args
}

// extractImportPath extracts the import path from an arrow function with dynamic import
// e.g., () => import('path') -> 'path'
func extractImportPath(arrowFunc *tree_sitter.Node, content []byte) string {
	// Find the call_expression with import
	importCallPattern := treesitterhelper.And(
		treesitterhelper.NodeKind("call_expression"),
		treesitterhelper.HasChild(treesitterhelper.NodeKind("import")),
	)

	importCall := treesitterhelper.FindFirst(arrowFunc, importCallPattern, content)
	if importCall == nil {
		return ""
	}

	// Find the string argument
	stringFragmentPattern := treesitterhelper.NodeKind("string_fragment")
	fragment := treesitterhelper.FindFirst(importCall, stringFragmentPattern, content)
	if fragment == nil {
		return ""
	}

	return string(fragment.Utf8Text(content))
}

// parseInlineDefinition parses an inline component definition object
func parseInlineDefinition(objNode *tree_sitter.Node, content []byte, filePath string) *ComponentDefinition {
	def := &ComponentDefinition{
		FilePath: filePath,
	}

	// Parse the object properties
	for i := uint(0); i < objNode.ChildCount(); i++ {
		child := objNode.Child(i)

		switch child.Kind() {
		case "pair":
			parseDefinitionPair(child, content, def)
		case "shorthand_property_identifier":
			// Handle shorthand like `template,`
			name := string(child.Utf8Text(content))
			if name == "template" {
				def.HasTemplate = true
			}
		case "method_definition":
			// Handle method shorthand like `data() { ... }`
			propIdent := treesitterhelper.GetFirstNodeOfKind(child, "property_identifier")
			if propIdent != nil {
				methodName := string(propIdent.Utf8Text(content))
				// data, created, mounted etc. are lifecycle methods, not regular methods
				// We could add them to a separate list if needed
				switch methodName {
				case "data", "created", "mounted", "updated", "destroyed", "beforeCreate",
					"beforeMount", "beforeUpdate", "beforeDestroy", "setup":
					// Lifecycle hooks - ignore for now
				default:
					// Could be a method defined at top level (unusual but valid)
				}
			}
		}
	}

	return def
}

// parseDefinitionPair parses a key-value pair in an inline component definition
func parseDefinitionPair(node *tree_sitter.Node, content []byte, def *ComponentDefinition) {
	// Get property name
	propIdent := treesitterhelper.GetFirstNodeOfKind(node, "property_identifier")
	if propIdent == nil {
		return
	}
	propName := string(propIdent.Utf8Text(content))

	// Get value node
	var valueNode *tree_sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		kind := child.Kind()
		if kind == "object" || kind == "array" || kind == "identifier" {
			valueNode = child
			break
		}
	}

	if valueNode == nil {
		return
	}

	switch propName {
	case "props":
		def.Props = parseProps(valueNode, content)
	case "emits":
		def.Emits = parseEmits(valueNode, content)
	case "methods":
		def.Methods = parseMethods(valueNode, content)
	case "computed":
		def.Computed = parseMethods(valueNode, content)
	case "template":
		def.HasTemplate = true
	}
}

// resolveImportPath resolves an import path relative to the registration file
func resolveImportPath(registrationFile, importPath string) string {
	if importPath == "" {
		return ""
	}

	var basePath string

	// If it starts with 'src/', it's an absolute path from the administration root
	if strings.HasPrefix(importPath, "src/") {
		// Find the administration root
		adminIdx := strings.Index(registrationFile, "Resources/app/administration/")
		if adminIdx != -1 {
			adminRoot := registrationFile[:adminIdx+len("Resources/app/administration/")]
			basePath = filepath.Join(adminRoot, importPath)
		} else {
			return importPath
		}
	} else if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		// Handle relative paths
		dir := filepath.Dir(registrationFile)
		basePath = filepath.Join(dir, importPath)
	} else {
		return importPath
	}

	// Try to resolve the actual file
	return resolveJSFile(basePath)
}

// resolveJSFile tries to find the actual JS/TS file for an import path
// It checks for: path.js, path.ts, path/index.js, path/index.ts
func resolveJSFile(basePath string) string {
	// If already has extension, return as-is
	if strings.HasSuffix(basePath, ".js") || strings.HasSuffix(basePath, ".ts") {
		return basePath
	}

	// Try direct file with extensions
	candidates := []string{
		basePath + ".js",
		basePath + ".ts",
		filepath.Join(basePath, "index.js"),
		filepath.Join(basePath, "index.ts"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback: return with /index.js as most common pattern
	return filepath.Join(basePath, "index.js")
}
