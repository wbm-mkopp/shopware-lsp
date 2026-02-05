package admin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// slotTagPattern matches <slot> and <slot name="..."> tags
// Captures the slot name from the name attribute if present
var slotTagPattern = regexp.MustCompile(`<slot(?:\s+name=["']([^"']+)["'])?[^>]*>`)

// twigBlockPattern matches {% block block_name %} tags
// Captures the block name
var twigBlockPattern = regexp.MustCompile(`\{%\s*block\s+(\w+)\s*%\}`)

// TemplateParseResult contains slots and blocks extracted from a template
type TemplateParseResult struct {
	Slots  []VueComponentSlot
	Blocks []TwigBlock
}

// ParseSlotsFromTemplate parses slot definitions from a Twig template file
// It looks for <slot> and <slot name="..."> tags and returns slot info with line numbers
func ParseSlotsFromTemplate(templatePath string) ([]VueComponentSlot, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	result := parseTemplateContent(string(content))
	return result.Slots, nil
}

// ParseTemplateFromFile parses both slots and blocks from a Twig template file
func ParseTemplateFromFile(templatePath string) (*TemplateParseResult, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	result := parseTemplateContent(string(content))
	return &result, nil
}

// parseTemplateContent extracts slots and blocks from template content
func parseTemplateContent(content string) TemplateParseResult {
	var result TemplateParseResult
	seenSlots := make(map[string]bool)
	seenBlocks := make(map[string]bool)

	// Split content into lines for line number tracking
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		// Extract slots
		slotMatches := slotTagPattern.FindAllStringSubmatch(line, -1)
		for _, match := range slotMatches {
			slotName := "default"
			if len(match) > 1 && match[1] != "" {
				slotName = match[1]
			}

			if !seenSlots[slotName] {
				seenSlots[slotName] = true
				result.Slots = append(result.Slots, VueComponentSlot{
					Name: slotName,
					Line: lineNum + 1, // 1-based line number
				})
			}
		}

		// Extract blocks
		blockMatches := twigBlockPattern.FindAllStringSubmatch(line, -1)
		for _, match := range blockMatches {
			if len(match) > 1 && match[1] != "" {
				blockName := match[1]
				if !seenBlocks[blockName] {
					seenBlocks[blockName] = true
					result.Blocks = append(result.Blocks, TwigBlock{
						Name: blockName,
						Line: lineNum + 1, // 1-based line number
					})
				}
			}
		}
	}

	return result
}

// parseSlotsFromContent extracts slot names and line numbers from template content (for tests)
func parseSlotsFromContent(content string) []VueComponentSlot {
	return parseTemplateContent(content).Slots
}

// ResolveTemplatePath resolves the template import path to an absolute file path
// relative to the component definition file
func ResolveTemplatePath(definitionPath, templateImport string) string {
	if templateImport == "" {
		return ""
	}

	dir := filepath.Dir(definitionPath)

	// Handle relative paths
	if strings.HasPrefix(templateImport, "./") || strings.HasPrefix(templateImport, "../") {
		return filepath.Join(dir, templateImport)
	}

	// If it's just a filename, assume it's in the same directory
	return filepath.Join(dir, templateImport)
}
