package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func parseJS(t *testing.T, code string) *tree_sitter.Node {
	parser := tree_sitter.NewParser()
	t.Cleanup(func() { parser.Close() })

	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))

	tree := parser.Parse([]byte(code), nil)
	t.Cleanup(func() { tree.Close() })

	return tree.RootNode()
}

func TestParseComponentDefinition_Props(t *testing.T) {
	code := `
export default {
    props: {
        title: {
            type: String,
            required: true,
        },
        count: {
            type: Number,
            default: 0,
        },
        active: Boolean,
    },
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	require.Len(t, def.Props, 3)

	assert.Equal(t, "title", def.Props[0].Name)
	assert.Equal(t, "String", def.Props[0].Type)
	assert.True(t, def.Props[0].Required)

	assert.Equal(t, "count", def.Props[1].Name)
	assert.Equal(t, "Number", def.Props[1].Type)
	assert.False(t, def.Props[1].Required)
	assert.Equal(t, "0", def.Props[1].Default)

	assert.Equal(t, "active", def.Props[2].Name)
	assert.Equal(t, "Boolean", def.Props[2].Type)
}

func TestParseComponentDefinition_PropsArray(t *testing.T) {
	code := `
export default {
    props: ['title', 'count', 'active'],
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	require.Len(t, def.Props, 3)
	assert.Equal(t, "title", def.Props[0].Name)
	assert.Equal(t, "count", def.Props[1].Name)
	assert.Equal(t, "active", def.Props[2].Name)
}

func TestParseComponentDefinition_Emits(t *testing.T) {
	code := `
export default {
    emits: ['filter-reset', 'update:modelValue', 'close'],
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	require.Len(t, def.Emits, 3)
	assert.Equal(t, "filter-reset", def.Emits[0])
	assert.Equal(t, "update:modelValue", def.Emits[1])
	assert.Equal(t, "close", def.Emits[2])
}

func TestParseComponentDefinition_Methods(t *testing.T) {
	code := `
export default {
    methods: {
        resetFilter() {
            this.$emit('filter-reset');
        },
        handleClick() {
            console.log('clicked');
        },
    },
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	require.Len(t, def.Methods, 2)
	assert.Equal(t, "resetFilter", def.Methods[0])
	assert.Equal(t, "handleClick", def.Methods[1])
}

func TestParseComponentDefinition_Computed(t *testing.T) {
	code := `
export default {
    computed: {
        fullName() {
            return this.firstName + ' ' + this.lastName;
        },
        isActive() {
            return this.status === 'active';
        },
    },
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	require.Len(t, def.Computed, 2)
	assert.Equal(t, "fullName", def.Computed[0])
	assert.Equal(t, "isActive", def.Computed[1])
}

func TestParseComponentDefinition_TemplateImport(t *testing.T) {
	code := `
import template from './sw-base-filter.html.twig';
import './sw-base-filter.scss';

export default {
    template,
    props: {},
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	assert.Equal(t, "./sw-base-filter.html.twig", def.TemplatePath)
	assert.True(t, def.HasTemplate)
}

func TestParseComponentDefinition_Full(t *testing.T) {
	code := `
import template from './sw-base-filter.html.twig';
import './sw-base-filter.scss';

export default {
    template,

    emits: ['filter-reset'],

    props: {
        title: {
            type: String,
            required: true,
        },
        showResetButton: {
            type: Boolean,
            required: true,
        },
        active: {
            type: Boolean,
            required: true,
        },
    },

    computed: {
        isVisible() {
            return this.active && this.showResetButton;
        },
    },

    methods: {
        resetFilter() {
            this.$emit('filter-reset');
        },
    },
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	assert.Equal(t, "./sw-base-filter.html.twig", def.TemplatePath)
	assert.True(t, def.HasTemplate)

	require.Len(t, def.Emits, 1)
	assert.Equal(t, "filter-reset", def.Emits[0])

	require.Len(t, def.Props, 3)
	assert.Equal(t, "title", def.Props[0].Name)
	assert.Equal(t, "showResetButton", def.Props[1].Name)
	assert.Equal(t, "active", def.Props[2].Name)

	require.Len(t, def.Computed, 1)
	assert.Equal(t, "isVisible", def.Computed[0])

	require.Len(t, def.Methods, 1)
	assert.Equal(t, "resetFilter", def.Methods[0])
}

func TestParseComponentDefinition_NoExportDefault(t *testing.T) {
	code := `
const component = {
    props: {
        title: String,
    },
};
`
	root := parseJS(t, code)
	def := ParseComponentDefinition(root, []byte(code))

	// Should return empty definition when no export default
	assert.Empty(t, def.Props)
	assert.Empty(t, def.Methods)
}
