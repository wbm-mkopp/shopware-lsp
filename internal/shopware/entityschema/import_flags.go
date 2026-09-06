package entityschema

import (
	"sort"
	"strconv"
	"strings"

	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

type importedFlagSet struct {
	required                 bool
	primary                  bool
	apiAware                 bool
	apiAwareSources          []string
	ranking                  float64
	rankingTokenize          *bool
	behavior                 *FieldBehavior
	metadata                 *FieldMetadata
	inherited                bool
	inheritedForeignKey      string
	reverseInheritedProperty string
	preserved                []string
}

type importedModifierSet struct {
	beforeFlags []string
	afterFlags  []string
}

func importedModifiers(value, creation *phpsyntax.Node) importedModifierSet {
	calls := phpquery.Calls(value)
	sort.SliceStable(calls, func(i, j int) bool {
		if calls[i].Range().End != calls[j].Range().End {
			return calls[i].Range().End < calls[j].Range().End
		}
		return calls[i].Range().Start > calls[j].Range().Start
	})
	var result importedModifierSet
	seenFlags := false
	for _, call := range calls {
		if call.Kind() != phpsyntax.PhpMemberCall || call.Range().Start > creation.Range().Start || call.Range().End < creation.Range().End {
			continue
		}
		method := phpquery.CallMethodName(call)
		if method == "addFlags" {
			seenFlags = true
			continue
		}
		suffix := importedCallSuffix(call, method)
		if suffix == "" {
			continue
		}
		if seenFlags {
			result.afterFlags = append(result.afterFlags, suffix)
		} else {
			result.beforeFlags = append(result.beforeFlags, suffix)
		}
	}
	return result
}

func importedCallSuffix(call *phpsyntax.Node, method string) string {
	if method == "" {
		return ""
	}
	text := call.Text()
	index := strings.LastIndex(text, "->"+method)
	if index < 0 {
		return ""
	}
	return dedentInlinePHPExpression(text[index:])
}

func withFieldModifiers(field FieldSpec, modifiers importedModifierSet) FieldSpec {
	field.ModifiersBeforeFlags = append([]string(nil), modifiers.beforeFlags...)
	field.ModifiersAfterFlags = append([]string(nil), modifiers.afterFlags...)
	return field
}

func withAssociationModifiers(field FieldSpec, modifiers importedModifierSet) FieldSpec {
	field.AssociationBeforeFlags = append([]string(nil), modifiers.beforeFlags...)
	field.AssociationAfterFlags = append([]string(nil), modifiers.afterFlags...)
	return field
}

func importedFlags(value, fieldCreation *phpsyntax.Node, resolve func(string) string) importedFlagSet {
	var result importedFlagSet
	for _, call := range phpquery.Calls(value) {
		if call.Kind() != phpsyntax.PhpMemberCall || phpquery.CallMethodName(call) != "addFlags" ||
			call.Range().Start > fieldCreation.Range().Start || call.Range().End < fieldCreation.Range().End {
			continue
		}
		for index := range phpquery.Arguments(call) {
			source := strings.TrimSpace(phpquery.ArgumentValueText(call, index))
			expression := phpquery.ArgumentExpression(call, index)
			creation := outerObjectCreation(expression)
			if creation == nil {
				if source != "" {
					result.preserved = append(result.preserved, source)
				}
				continue
			}
			importFlagCreation(&result, creation, source, resolve)
		}
	}
	return result
}

func outerObjectCreation(expression *phpsyntax.Node) *phpsyntax.Node {
	creations := phpquery.ObjectCreations(expression)
	if len(creations) == 0 {
		return nil
	}
	outer := creations[0]
	for _, creation := range creations[1:] {
		if creation.Range().End-creation.Range().Start > outer.Range().End-outer.Range().Start {
			outer = creation
		}
	}
	return outer
}

func importFlagCreation(result *importedFlagSet, creation *phpsyntax.Node, source string, resolve func(string) string) {
	preserve := func() {
		if source == "" {
			source = strings.TrimSpace(creation.Text())
		}
		result.preserved = append(result.preserved, source)
	}
	switch ShortClass(phpquery.ObjectClassName(creation)) {
	case "Required":
		result.required = true
	case "PrimaryKey":
		result.primary = true
	case "ApiAware":
		sources, recognized := importedAPIAwareSources(creation, resolve)
		if !recognized {
			preserve()
			return
		}
		result.apiAware = true
		result.apiAwareSources = sources
	case "SearchRanking":
		ranking, tokenize, recognized := importedSearchRanking(creation)
		if !recognized {
			preserve()
			return
		}
		result.ranking = ranking
		result.rankingTokenize = tokenize
	case "Runtime":
		behavior := ensureImportedBehavior(result)
		behavior.Runtime = true
		behavior.RuntimeDependencies, behavior.RuntimeDependenciesExpression = importedRuntimeDependencies(creation)
	case "Computed":
		ensureImportedBehavior(result).Computed = true
	case "NoConstraint":
		ensureImportedBehavior(result).NoConstraint = true
	case "AllowHtml":
		value, recognized := importedDefaultBoolArgument(creation, 0, true)
		if !recognized {
			preserve()
			return
		}
		ensureImportedMetadata(result).AllowHTML = &value
	case "AllowEmptyString":
		ensureImportedMetadata(result).AllowEmptyString = true
	case "AsArray":
		ensureImportedMetadata(result).AsArray = true
	case "Immutable":
		ensureImportedMetadata(result).Immutable = true
	case "Since":
		if value := importedStringArgument(creation, 0); value != "" {
			ensureImportedMetadata(result).Since = value
		} else {
			preserve()
		}
	case "Deprecated":
		if deprecated, recognized := importedDeprecation(creation); recognized {
			ensureImportedMetadata(result).Deprecated = deprecated
		} else {
			preserve()
		}
	case "IgnoreInOpenapiSchema":
		ensureImportedMetadata(result).IgnoreInOpenAPISchema = true
	case "IgnoreInUnusedMediaSearch":
		ensureImportedMetadata(result).IgnoreInUnusedMediaSearch = true
	case "ApiCriteriaAware":
		ensureImportedMetadata(result).APICriteriaAware = true
	case "RuleAreas":
		if areas, recognized := importedRuleAreas(creation); recognized {
			ensureImportedMetadata(result).RuleAreas = areas
		} else {
			preserve()
		}
	case "Choice":
		if choice, recognized := importedChoice(creation); recognized {
			ensureImportedMetadata(result).Choice = choice
		} else {
			preserve()
		}
	case "DoNotUseContext":
		ensureImportedMetadata(result).DoNotUseContext = true
	case "Extension":
		ensureImportedMetadata(result).Extension = true
	case "Inherited":
		result.inherited = true
		result.inheritedForeignKey = importedStringArgument(creation, 0)
	case "ReverseInherited":
		result.reverseInheritedProperty = importedStringArgument(creation, 0)
	case "CascadeDelete", "SetNullOnDelete", "RestrictDelete":
		// Represented structurally by the field kind or relation behavior.
	default:
		preserve()
	}
}

func ensureImportedBehavior(flags *importedFlagSet) *FieldBehavior {
	if flags.behavior == nil {
		flags.behavior = &FieldBehavior{}
	}
	return flags.behavior
}

func ensureImportedMetadata(flags *importedFlagSet) *FieldMetadata {
	if flags.metadata == nil {
		flags.metadata = &FieldMetadata{}
	}
	return flags.metadata
}

func importedDefaultBoolArgument(creation *phpsyntax.Node, index int, fallback bool) (bool, bool) {
	arguments := phpquery.Arguments(creation)
	if index < 0 || index >= len(arguments) {
		return fallback, true
	}
	value := strings.ToLower(strings.TrimSpace(phpquery.ArgumentValueText(creation, index)))
	if value != "true" && value != "false" {
		return false, false
	}
	return value == "true", true
}

func importedAPIAwareSources(creation *phpsyntax.Node, resolve func(string) string) ([]string, bool) {
	arguments := phpquery.Arguments(creation)
	if len(arguments) == 0 {
		return nil, true
	}
	sources := make([]string, 0, len(arguments))
	for index := range arguments {
		class := importedClassArgument(creation, index)
		if class == "" {
			class = importedStringArgument(creation, index)
		}
		class = strings.Trim(resolve(class), `\ `)
		if class == "" {
			return nil, false
		}
		sources = appendUniqueStrings(sources, class)
	}
	return sources, true
}

// normalizeImportedPHPExpression makes class references self-contained so a
// generated definition does not depend on aliases from the source file's old
// use list. Other syntax remains byte-for-byte unchanged.
func normalizeImportedPHPExpression(source string, resolve func(string) string) string {
	source = dedentInlinePHPExpression(source)
	qualify := func(class string) string {
		if strings.HasPrefix(strings.TrimSpace(class), `\`) {
			return `\` + strings.Trim(class, `\ `)
		}
		switch strings.ToLower(strings.TrimSpace(class)) {
		case "", "self", "static", "parent", "class":
			return class
		}
		resolved := strings.Trim(resolve(class), `\ `)
		if resolved == "" {
			return class
		}
		return `\` + resolved
	}
	source = importedNewClassPattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := importedNewClassPattern.FindStringSubmatch(match)
		return parts[1] + qualify(parts[2])
	})
	return importedScopedRefPattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := importedScopedRefPattern.FindStringSubmatch(match)
		return qualify(parts[1]) + "::" + parts[2]
	})
}

func dedentInlinePHPExpression(source string) string {
	lines := strings.Split(strings.TrimSpace(source), "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(source)
	}
	minimum := -1
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minimum < 0 || indent < minimum {
			minimum = indent
		}
	}
	if minimum > 0 {
		for index := 1; index < len(lines); index++ {
			if len(lines[index]) >= minimum {
				lines[index] = lines[index][minimum:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func importedDeprecation(creation *phpsyntax.Node) (*Deprecation, bool) {
	arguments := phpquery.Arguments(creation)
	if len(arguments) < 2 || len(arguments) > 3 {
		return nil, false
	}
	deprecated := &Deprecation{
		DeprecatedSince: importedStringArgument(creation, 0),
		WillBeRemovedIn: importedStringArgument(creation, 1),
	}
	if len(arguments) == 3 {
		deprecated.ReplacedBy = importedStringArgument(creation, 2)
	}
	return deprecated, deprecated.DeprecatedSince != "" && deprecated.WillBeRemovedIn != ""
}

func importedRuleAreas(creation *phpsyntax.Node) ([]string, bool) {
	arguments := phpquery.Arguments(creation)
	areas := make([]string, 0, len(arguments))
	for index := range arguments {
		expression := phpquery.ArgumentExpression(creation, index)
		if expression != nil && expression.Kind() == phpsyntax.PhpString {
			areas = append(areas, phpquery.StringValue(expression))
			continue
		}
		normalized := strings.ReplaceAll(strings.TrimSpace(phpquery.ArgumentValueText(creation, index)), " ", "")
		separator := strings.LastIndex(normalized, "::")
		if separator < 0 {
			return nil, false
		}
		constant := normalized[separator+2:]
		switch constant {
		case "PRODUCT_AREA":
			areas = append(areas, "product")
		case "PAYMENT_AREA":
			areas = append(areas, "payment")
		case "SHIPPING_AREA":
			areas = append(areas, "shipping")
		case "PROMOTION_AREA":
			areas = append(areas, "promotion")
		case "FLOW_AREA":
			areas = append(areas, "flow")
		case "FLOW_CONDITION_AREA":
			areas = append(areas, "flow-condition")
		case "CATEGORY_AREA":
			areas = append(areas, "category")
		case "LANDING_PAGE_AREA":
			areas = append(areas, "landing-page")
		default:
			return nil, false
		}
	}
	return areas, true
}

func importedChoice(creation *phpsyntax.Node) (*ChoiceSpec, bool) {
	arguments := phpquery.Arguments(creation)
	if len(arguments) == 0 || len(arguments) > 2 {
		return nil, false
	}
	expression := phpquery.ArgumentExpression(creation, 0)
	if expression == nil {
		return nil, false
	}
	arrays := phpquery.Arrays(expression)
	if len(arrays) != 1 || arrays[0].Range() != expression.Range() {
		return nil, false
	}
	choice := &ChoiceSpec{}
	for _, item := range phpquery.ArrayItems(arrays[0]) {
		value := phpquery.ArrayItemValue(item)
		if value == nil {
			return nil, false
		}
		choice.Values = append(choice.Values, strings.TrimSpace(value.Text()))
	}
	if len(arguments) == 2 {
		strict, recognized := importedDefaultBoolArgument(creation, 1, false)
		if !recognized {
			return nil, false
		}
		choice.Strict = &strict
	}
	return choice, true
}

func importedSearchRanking(creation *phpsyntax.Node) (float64, *bool, bool) {
	arguments := phpquery.Arguments(creation)
	if len(arguments) == 0 || len(arguments) > 2 {
		return 0, nil, false
	}
	rankingExpression := strings.TrimSpace(phpquery.ArgumentValueText(creation, 0))
	ranking, err := strconv.ParseFloat(rankingExpression, 64)
	if err != nil {
		normalized := strings.ReplaceAll(strings.TrimPrefix(rankingExpression, `\`), " ", "")
		switch {
		case strings.HasSuffix(normalized, "SearchRanking::ASSOCIATION_SEARCH_RANKING"):
			ranking = 0.25
		case strings.HasSuffix(normalized, "SearchRanking::LOW_SEARCH_RANKING"):
			ranking = 80
		case strings.HasSuffix(normalized, "SearchRanking::MIDDLE_SEARCH_RANKING"):
			ranking = 250
		case strings.HasSuffix(normalized, "SearchRanking::HIGH_SEARCH_RANKING"):
			ranking = 500
		default:
			return 0, nil, false
		}
	}
	if len(arguments) == 1 {
		return ranking, nil, true
	}
	value := strings.ToLower(strings.TrimSpace(phpquery.ArgumentValueText(creation, 1)))
	if value != "true" && value != "false" {
		return 0, nil, false
	}
	tokenize := value == "true"
	return ranking, &tokenize, true
}

func importedRuntimeDependencies(creation *phpsyntax.Node) ([]string, string) {
	arguments := phpquery.Arguments(creation)
	if len(arguments) == 0 {
		return nil, ""
	}
	expression := phpquery.ArgumentExpression(creation, 0)
	if expression == nil {
		return nil, strings.TrimSpace(phpquery.ArgumentValueText(creation, 0))
	}
	arrays := phpquery.Arrays(expression)
	if len(arrays) != 1 || arrays[0].Range() != expression.Range() {
		return nil, strings.TrimSpace(expression.Text())
	}
	items := phpquery.ArrayItems(arrays[0])
	dependencies := make([]string, 0, len(items))
	for _, item := range items {
		value := phpquery.ArrayItemValue(item)
		if value == nil || value.Kind() != phpsyntax.PhpString {
			return nil, strings.TrimSpace(expression.Text())
		}
		dependencies = append(dependencies, phpquery.StringValue(value))
	}
	return dependencies, ""
}
