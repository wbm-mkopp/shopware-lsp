package admin

// VueComponent represents a Shopware 6 Admin Vue component
type VueComponent struct {
	// Name is the component name (e.g., "sw-base-filter")
	Name string

	// ExtendsComponent is the parent component name if this component extends another (empty for register)
	ExtendsComponent string

	// ImportPath is the path from the import statement (e.g., "src/app/component/filter/sw-base-filter/index")
	ImportPath string

	// FilePath is the absolute path to the registration file
	FilePath string

	// DefinitionPath is the resolved absolute path to the component definition file
	DefinitionPath string

	// Line is the line number where the component is registered
	Line int

	// Props contains the component's props definitions
	Props []VueComponentProp

	// Emits contains the component's emitted events
	Emits []string

	// Methods contains the component's method names
	Methods []string

	// Computed contains the component's computed property names
	Computed []string

	// Slots contains the component's slot definitions
	Slots []VueComponentSlot

	// Blocks contains the component's Twig block definitions
	Blocks []TwigBlock

	// TemplatePath is the path to the Twig template (from the template import)
	TemplatePath string

	// InlineDefinition contains the parsed definition for inline component registrations
	// This is only populated during indexing and not persisted (used to store in definition index)
	InlineDefinition *ComponentDefinition `msgpack:"-"`
}

// VueComponentProp represents a prop definition in a Vue component
type VueComponentProp struct {
	// Name is the prop name
	Name string

	// Type is the prop type (e.g., "String", "Boolean", "Object")
	Type string

	// Required indicates if the prop is required
	Required bool

	// Default is the default value (as string representation)
	Default string

	// Line is the line number where the prop is defined (1-based)
	Line int
}

// VueComponentSlot represents a slot definition in a Vue component template
type VueComponentSlot struct {
	// Name is the slot name (e.g., "default", "actions", "header")
	Name string

	// Line is the line number where the slot is defined in the template (1-based)
	Line int
}

// TwigBlock represents a Twig block definition in a component template
type TwigBlock struct {
	// Name is the block name (e.g., "sw_page_content", "sw_page_smart_bar")
	Name string

	// Line is the line number where the block is defined in the template (1-based)
	Line int
}
