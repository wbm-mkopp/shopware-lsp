package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func TestParseComponentRegister(t *testing.T) {
	code := `
Shopware.Component.register('sw-base-filter', () => import('src/app/component/filter/sw-base-filter/index'));
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/app/component/index.ts"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 1)
	assert.Equal(t, "sw-base-filter", components[0].Name)
	assert.Equal(t, "", components[0].ExtendsComponent)
	assert.Equal(t, "src/app/component/filter/sw-base-filter/index", components[0].ImportPath)
	assert.Equal(t, filePath, components[0].FilePath)
	assert.Equal(t, 2, components[0].Line)
}

func TestParseComponentExtend(t *testing.T) {
	code := `
Shopware.Component.extend('sw-condition-time-range', 'sw-condition-base', () => import('./rule/condition-type/sw-condition-time-range/index'));
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/app/component/index.ts"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 1)
	assert.Equal(t, "sw-condition-time-range", components[0].Name)
	assert.Equal(t, "sw-condition-base", components[0].ExtendsComponent)
	assert.Equal(t, "./rule/condition-type/sw-condition-time-range/index", components[0].ImportPath)
}

func TestParseMultipleComponents(t *testing.T) {
	code := `
Shopware.Component.register('sw-wizard-page', () => import('src/app/component/wizard/sw-wizard-page/index'));
Shopware.Component.register('sw-wizard', () => import('src/app/component/wizard/sw-wizard/index'));
Shopware.Component.extend('sw-sidebar-collapse', 'sw-collapse', () => import('./sidebar/sw-sidebar-collapse/index'));
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/app/component/index.ts"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 3)

	assert.Equal(t, "sw-wizard-page", components[0].Name)
	assert.Equal(t, "", components[0].ExtendsComponent)

	assert.Equal(t, "sw-wizard", components[1].Name)
	assert.Equal(t, "", components[1].ExtendsComponent)

	assert.Equal(t, "sw-sidebar-collapse", components[2].Name)
	assert.Equal(t, "sw-collapse", components[2].ExtendsComponent)
}

func TestResolveImportPath(t *testing.T) {
	tests := []struct {
		name             string
		registrationFile string
		importPath       string
		expected         string
	}{
		{
			name:             "absolute src path with index",
			registrationFile: "/project/src/Administration/Resources/app/administration/src/app/component/index.ts",
			importPath:       "src/app/component/filter/sw-base-filter/index",
			// Has extension-like suffix, so gets .js appended
			expected: "/project/src/Administration/Resources/app/administration/src/app/component/filter/sw-base-filter/index/index.js",
		},
		{
			name:             "relative path with index",
			registrationFile: "/project/src/Administration/Resources/app/administration/src/app/component/index.ts",
			importPath:       "./filter/sw-base-filter/index",
			expected:         "/project/src/Administration/Resources/app/administration/src/app/component/filter/sw-base-filter/index/index.js",
		},
		{
			name:             "parent relative path - directory import",
			registrationFile: "/project/src/Administration/Resources/app/administration/src/module/sw-settings/index.js",
			importPath:       "../sw-other/component",
			// Falls back to /index.js since file doesn't exist
			expected: "/project/src/Administration/Resources/app/administration/src/module/sw-other/component/index.js",
		},
		{
			name:             "import with .js extension",
			registrationFile: "/project/src/Administration/Resources/app/administration/src/app/component/index.ts",
			importPath:       "./filter/sw-base-filter.js",
			expected:         "/project/src/Administration/Resources/app/administration/src/app/component/filter/sw-base-filter.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveImportPath(tt.registrationFile, tt.importPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIndexerFiltering(t *testing.T) {
	code := `
Shopware.Component.register('sw-base-filter', () => import('src/app/component/filter/sw-base-filter/index'));
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	// Non-administration path should return empty
	nonAdminPath := "/project/src/Storefront/Resources/app/storefront/src/main.js"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), nonAdminPath)

	// The parsing still works, but the indexer filters by path in Index()
	// So here we just test that parsing works regardless of path
	require.Len(t, components, 1)
}

func TestParseDestructuredComponent(t *testing.T) {
	code := `
const { Component } = Shopware;

Component.register('my-component', {
    template,
    props: {
        title: String,
    },
    methods: {
        handleClick() {}
    },
});
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/module/my-module/component/my-component/index.js"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 1)
	assert.Equal(t, "my-component", components[0].Name)
	assert.Equal(t, "", components[0].ExtendsComponent)
	assert.Equal(t, filePath, components[0].DefinitionPath) // Inline definition, same file

	// Check inline definition was parsed
	require.NotNil(t, components[0].InlineDefinition)
	assert.True(t, components[0].InlineDefinition.HasTemplate)
	require.Len(t, components[0].InlineDefinition.Props, 1)
	assert.Equal(t, "title", components[0].InlineDefinition.Props[0].Name)
	assert.Equal(t, "String", components[0].InlineDefinition.Props[0].Type)
	require.Len(t, components[0].InlineDefinition.Methods, 1)
	assert.Equal(t, "handleClick", components[0].InlineDefinition.Methods[0])
}

func TestParseInlineComponentDefinition(t *testing.T) {
	code := `
Shopware.Component.register('inline-component', {
    template,
    
    emits: ['change', 'submit'],
    
    props: {
        name: {
            type: String,
            required: true,
        },
        count: {
            type: Number,
            default: 0,
        },
    },
    
    computed: {
        fullName() {
            return this.name;
        },
    },
    
    methods: {
        save() {},
        cancel() {},
    },
});
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/component/index.js"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 1)
	assert.Equal(t, "inline-component", components[0].Name)

	def := components[0].InlineDefinition
	require.NotNil(t, def)

	assert.True(t, def.HasTemplate)

	require.Len(t, def.Emits, 2)
	assert.Equal(t, "change", def.Emits[0])
	assert.Equal(t, "submit", def.Emits[1])

	require.Len(t, def.Props, 2)
	assert.Equal(t, "name", def.Props[0].Name)
	assert.Equal(t, "String", def.Props[0].Type)
	assert.True(t, def.Props[0].Required)

	assert.Equal(t, "count", def.Props[1].Name)
	assert.Equal(t, "Number", def.Props[1].Type)
	assert.Equal(t, "0", def.Props[1].Default)

	require.Len(t, def.Computed, 1)
	assert.Equal(t, "fullName", def.Computed[0])

	require.Len(t, def.Methods, 2)
	assert.Equal(t, "save", def.Methods[0])
	assert.Equal(t, "cancel", def.Methods[1])
}

func TestParseExtendWithInlineDefinition(t *testing.T) {
	code := `
const { Component } = Shopware;

Component.extend('my-extended', 'sw-base', {
    props: {
        extra: Boolean,
    },
    methods: {
        customMethod() {},
    },
});
`
	parser := tree_sitter.NewParser()
	defer parser.Close()

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	filePath := "/project/src/Administration/Resources/app/administration/src/module/my-module/index.js"
	components := parseComponentRegistrations(tree.RootNode(), []byte(code), filePath)

	require.Len(t, components, 1)
	assert.Equal(t, "my-extended", components[0].Name)
	assert.Equal(t, "sw-base", components[0].ExtendsComponent)

	def := components[0].InlineDefinition
	require.NotNil(t, def)

	require.Len(t, def.Props, 1)
	assert.Equal(t, "extra", def.Props[0].Name)
	assert.Equal(t, "Boolean", def.Props[0].Type)

	require.Len(t, def.Methods, 1)
	assert.Equal(t, "customMethod", def.Methods[0])
}

func TestAdminComponentIndexer(t *testing.T) {
	tempDir := t.TempDir()

	indexer, err := NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = indexer.Close() }()

	// Create a parser
	parser := tree_sitter.NewParser()
	defer parser.Close()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	// Index a registration file
	registrationCode := `
Shopware.Component.register('sw-test-component', () => import('src/app/component/test/sw-test-component/index'));
`
	registrationPath := "/project/src/Administration/Resources/app/administration/src/app/component/index.ts"

	tree := parser.Parse([]byte(registrationCode), nil)
	err = indexer.Index(registrationPath, tree.RootNode(), []byte(registrationCode))
	require.NoError(t, err)
	tree.Close()

	// Index a definition file
	definitionCode := `
import template from './sw-test-component.html.twig';

export default {
    template,
    
    emits: ['change', 'submit'],
    
    props: {
        title: {
            type: String,
            required: true,
        },
        disabled: {
            type: Boolean,
            default: false,
        },
    },
    
    computed: {
        isActive() {
            return !this.disabled;
        },
    },
    
    methods: {
        handleClick() {
            this.$emit('change');
        },
        submit() {
            this.$emit('submit');
        },
    },
};
`
	definitionPath := "/project/src/Administration/Resources/app/administration/src/app/component/test/sw-test-component/index.js"

	tree = parser.Parse([]byte(definitionCode), nil)
	err = indexer.Index(definitionPath, tree.RootNode(), []byte(definitionCode))
	require.NoError(t, err)
	tree.Close()

	// Check component was registered
	components, err := indexer.GetComponent("sw-test-component")
	require.NoError(t, err)
	require.Len(t, components, 1)
	assert.Equal(t, "sw-test-component", components[0].Name)

	// Check definition was indexed
	def, err := indexer.GetComponentDefinition(definitionPath)
	require.NoError(t, err)
	require.NotNil(t, def)

	// TemplatePath should be resolved to absolute path
	assert.Equal(t, "/project/src/Administration/Resources/app/administration/src/app/component/test/sw-test-component/sw-test-component.html.twig", def.TemplatePath)
	assert.True(t, def.HasTemplate)

	require.Len(t, def.Emits, 2)
	assert.Equal(t, "change", def.Emits[0])
	assert.Equal(t, "submit", def.Emits[1])

	require.Len(t, def.Props, 2)
	assert.Equal(t, "title", def.Props[0].Name)
	assert.Equal(t, "String", def.Props[0].Type)
	assert.True(t, def.Props[0].Required)

	assert.Equal(t, "disabled", def.Props[1].Name)
	assert.Equal(t, "Boolean", def.Props[1].Type)
	assert.False(t, def.Props[1].Required)
	assert.Equal(t, "false", def.Props[1].Default)

	require.Len(t, def.Computed, 1)
	assert.Equal(t, "isActive", def.Computed[0])

	require.Len(t, def.Methods, 2)
	assert.Equal(t, "handleClick", def.Methods[0])
	assert.Equal(t, "submit", def.Methods[1])

	// Test GetComponentWithDefinition - but note the paths won't match in this test
	// because the registration uses 'src/app/...' which resolves differently
	allComponents, err := indexer.GetAllComponents()
	require.NoError(t, err)
	assert.Len(t, allComponents, 1)
}

func TestWrapComponentConfig(t *testing.T) {
	tempDir := t.TempDir()

	indexer, err := NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = indexer.Close() }()

	// Create a parser
	parser := tree_sitter.NewParser()
	defer parser.Close()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	// Index a wrapped component config file (like Meteor component wrappers)
	wrapCode := `
import { MtCard } from '@shopware-ag/meteor-component-library';
import template from './mt-card.html.twig';

export default Shopware.Component.wrapComponentConfig({
    template,

    components: {
        'mt-card-original': MtCard,
    },

    inheritAttrs: false,

    props: {
        positionIdentifier: {
            type: String,
            required: true,
            default: null,
        },
        title: {
            type: String,
        },
    },

    computed: {
        filteredSlots() {
            return this.$slots;
        },
    },

    methods: {
        getFilteredSlots() {
            return this.$slots;
        },
    },
});
`
	// Path follows pattern: mt-card/index.ts
	wrapPath := "/project/src/Administration/Resources/app/administration/src/app/component/meteor-wrapper/mt-card/index.ts"

	tree := parser.Parse([]byte(wrapCode), nil)
	err = indexer.Index(wrapPath, tree.RootNode(), []byte(wrapCode))
	require.NoError(t, err)
	tree.Close()

	// Check component was registered with derived name
	components, err := indexer.GetComponent("mt-card")
	require.NoError(t, err)
	require.Len(t, components, 1)
	assert.Equal(t, "mt-card", components[0].Name)
	assert.Equal(t, wrapPath, components[0].FilePath)
	assert.Equal(t, wrapPath, components[0].DefinitionPath)

	// Check props were parsed
	require.Len(t, components[0].Props, 2)
	assert.Equal(t, "positionIdentifier", components[0].Props[0].Name)
	assert.Equal(t, "String", components[0].Props[0].Type)
	assert.True(t, components[0].Props[0].Required)

	assert.Equal(t, "title", components[0].Props[1].Name)
	assert.Equal(t, "String", components[0].Props[1].Type)

	// Check computed was parsed
	require.Len(t, components[0].Computed, 1)
	assert.Equal(t, "filteredSlots", components[0].Computed[0])

	// Check methods were parsed
	require.Len(t, components[0].Methods, 1)
	assert.Equal(t, "getFilteredSlots", components[0].Methods[0])

	// Verify GetComponentWithDefinition works
	componentsWithDef, err := indexer.GetComponentWithDefinition("mt-card")
	require.NoError(t, err)
	require.Len(t, componentsWithDef, 1)
	assert.Equal(t, "mt-card", componentsWithDef[0].Name)
	assert.Len(t, componentsWithDef[0].Props, 2)
}

func TestDeriveComponentNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/path/to/mt-card/index.ts",
			expected: "mt-card",
		},
		{
			path:     "/path/to/mt-card/index.js",
			expected: "mt-card",
		},
		{
			path:     "/path/to/sw-button.js",
			expected: "sw-button",
		},
		{
			path:     "/path/to/sw-button.ts",
			expected: "sw-button",
		},
		{
			path:     "/path/to/component/my-component/index.ts",
			expected: "my-component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := deriveComponentNameFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeduplicateComponents(t *testing.T) {
	tests := []struct {
		name       string
		components []VueComponent
		wantLen    int
		wantProps  int
	}{
		{
			name:       "empty list",
			components: []VueComponent{},
			wantLen:    0,
			wantProps:  0,
		},
		{
			name: "single component",
			components: []VueComponent{
				{Name: "test", Props: []VueComponentProp{{Name: "prop1"}}},
			},
			wantLen:   1,
			wantProps: 1,
		},
		{
			name: "two components same name - prefer one with more props",
			components: []VueComponent{
				{Name: "test", FilePath: "/file1.ts"},
				{Name: "test", FilePath: "/file2.ts", Props: []VueComponentProp{{Name: "prop1"}, {Name: "prop2"}}},
			},
			wantLen:   1,
			wantProps: 2,
		},
		{
			name: "merge data from both components",
			components: []VueComponent{
				{Name: "test", FilePath: "/file1.ts", ExtendsComponent: "parent"},
				{Name: "test", FilePath: "/file2.ts", Props: []VueComponentProp{{Name: "prop1"}}, DefinitionPath: "/def.ts"},
			},
			wantLen:   1,
			wantProps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateComponents(tt.components)
			assert.Len(t, result, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Len(t, result[0].Props, tt.wantProps)
			}
		})
	}
}

func TestMergeComponents(t *testing.T) {
	fallback := VueComponent{
		Name:             "test",
		ExtendsComponent: "parent",
		ImportPath:       "/import/path",
		Props:            []VueComponentProp{{Name: "fallbackProp"}},
		Emits:            []string{"fallbackEmit"},
	}

	preferred := VueComponent{
		Name:           "test",
		DefinitionPath: "/def/path",
		Props:          []VueComponentProp{{Name: "preferredProp1"}, {Name: "preferredProp2"}},
		Methods:        []string{"method1"},
	}

	result := mergeComponents(fallback, preferred)

	// Should take preferred values when available
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, "/def/path", result.DefinitionPath)
	assert.Len(t, result.Props, 2)
	assert.Equal(t, "preferredProp1", result.Props[0].Name)
	assert.Len(t, result.Methods, 1)

	// Should take fallback values when preferred is empty
	assert.Equal(t, "parent", result.ExtendsComponent)
	assert.Equal(t, "/import/path", result.ImportPath)
	assert.Len(t, result.Emits, 1)
	assert.Equal(t, "fallbackEmit", result.Emits[0])
}
