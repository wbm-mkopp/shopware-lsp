package hover

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/admin"
	jssyntax "github.com/shopware/shopware-lsp/internal/parser/javascript/syntax"
)

func (p *AdminHoverProvider) isInComponentCall(node *jssyntax.Node) bool {
	target, found := admin.JavaScriptSymbolAt(node)
	return found && target.Kind == admin.AdminSymbolComponent
}

func (p *AdminHoverProvider) extractComponentName(node *jssyntax.Node) string {
	target, found := admin.JavaScriptSymbolAt(node)
	if !found || target.Kind != admin.AdminSymbolComponent {
		return ""
	}
	return target.Name
}

// buildHoverContent preserves catalog order and delegates each component's sections.
func (p *AdminHoverProvider) buildHoverContent(components []admin.VueComponent) string {
	var sb strings.Builder
	for i, component := range components {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		p.writeComponentHover(&sb, component)
	}
	return sb.String()
}

func (p *AdminHoverProvider) writeComponentHover(sb *strings.Builder, component admin.VueComponent) {
	fmt.Fprintf(sb, "## `%s`\n\n", component.Name)
	if component.Deprecated != "" {
		fmt.Fprintf(sb, "**Deprecated:** %s\n\n", component.Deprecated)
	}
	if component.ExtendsComponent != "" {
		fmt.Fprintf(sb, "**Extends**: `%s`\n\n", component.ExtendsComponent)
	}
	writeComponentHoverProps(sb, component.Props)
	writeComponentHoverEvents(sb, component.ComponentEvents())
	writeComponentHoverNames(sb, "Methods", component.Methods, "()")
	writeComponentHoverNames(sb, "Computed", component.Computed, "")
	writeComponentHoverNames(sb, "Data", component.Data, "")
	writeComponentHoverNames(sb, "Injected services", component.Injected, "")
	writeComponentHoverSlots(sb, component.Slots)
	p.writeComponentHoverLocation(sb, component)
}

func writeComponentHoverProps(sb *strings.Builder, props []admin.VueComponentProp) {
	if len(props) == 0 {
		return
	}
	sb.WriteString("### Props\n\n")
	for _, prop := range props {
		fmt.Fprintf(sb, "- `%s`", prop.Name)
		if prop.Type != "" {
			fmt.Fprintf(sb, ": **%s**", prop.Type)
		}
		if prop.Required {
			sb.WriteString(" *(required)*")
		}
		if prop.Deprecated != "" {
			sb.WriteString(" *(deprecated)*")
		}
		if prop.Default != "" {
			fmt.Fprintf(sb, " = `%s`", prop.Default)
		}
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
}

func writeComponentHoverEvents(sb *strings.Builder, events []admin.VueComponentEvent) {
	if len(events) == 0 {
		return
	}
	sb.WriteString("### Events\n\n")
	for _, event := range events {
		fmt.Fprintf(sb, "- `%s`", admin.CanonicalEventName(event.Name))
		if event.Type != "" {
			fmt.Fprintf(sb, ": `%s`", event.Type)
		}
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
}

func writeComponentHoverNames(sb *strings.Builder, heading string, names []string, suffix string) {
	if len(names) == 0 {
		return
	}
	fmt.Fprintf(sb, "### %s\n\n", heading)
	for _, name := range names {
		fmt.Fprintf(sb, "- `%s%s`\n", name, suffix)
	}
	sb.WriteByte('\n')
}

func writeComponentHoverSlots(sb *strings.Builder, slots []admin.VueComponentSlot) {
	if len(slots) == 0 {
		return
	}
	sb.WriteString("### Slots\n\n")
	for _, slot := range slots {
		fmt.Fprintf(sb, "- `%s`\n", slot.DisplayName())
	}
	sb.WriteByte('\n')
}

func (p *AdminHoverProvider) writeComponentHoverLocation(sb *strings.Builder, component admin.VueComponent) {
	if component.DefinitionPath != "" {
		fmt.Fprintf(sb, "*Defined in*: `%s`\n", p.makeRelativePath(component.DefinitionPath))
	} else if component.FilePath != "" {
		fmt.Fprintf(sb, "*Registered in*: `%s`\n", p.makeRelativePath(component.FilePath))
	}
}

func firstHoverOwner(values []*admin.VueComponent) *admin.VueComponent {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

// makeRelativePath converts an absolute path to a path relative to the project root
func (p *AdminHoverProvider) makeRelativePath(absPath string) string {
	if p.projectRoot == "" {
		return absPath
	}
	relPath, err := filepath.Rel(p.projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return relPath
}
