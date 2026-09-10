package entityschema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	phpparser "github.com/shopware/shopware-lsp/internal/parser/php"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	phpresolver "github.com/shopware/shopware-lsp/internal/php/resolver"
)

var (
	importedEntityNamePattern = regexp.MustCompile(`(?m)\bENTITY_NAME\s*=\s*['"]([^'"]+)['"]`)
	importedAcronymBoundary   = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	importedWordBoundary      = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	importedNewClassPattern   = regexp.MustCompile(`(?i)(\bnew\s+)([\\A-Za-z_][\\A-Za-z0-9_]*)`)
	importedScopedRefPattern  = regexp.MustCompile(`([\\A-Za-z_][\\A-Za-z0-9_]*)::([A-Za-z_][A-Za-z0-9_]*)\b`)
)

type RelationTarget struct {
	DefinitionClass  string                `json:"definitionClass"`
	DefinitionKind   DefinitionKind        `json:"definitionKind,omitempty"`
	EntityClass      string                `json:"entityClass"`
	CollectionClass  string                `json:"collectionClass"`
	EntityName       string                `json:"entityName"`
	FileURI          string                `json:"fileUri,omitempty"`
	Fields           []RelationTargetField `json:"fields,omitempty"`
	VersionAware     bool                  `json:"versionAware,omitempty"`
	InheritanceAware bool                  `json:"inheritanceAware,omitempty"`
}

type RelationTargetField struct {
	PropertyName string `json:"propertyName"`
	StorageName  string `json:"storageName"`
	Primary      bool   `json:"primary,omitempty"`
}

// RelationLookup resolves either a fully-qualified definition class or a
// technical entity name. Entity-name resolution is required for Shopware 6.7+
// EntityExtension implementations, whose only mandatory target method is
// getEntityName().
type RelationLookup func(classOrEntityName string) (RelationTarget, bool)

type ImportedTranslation struct {
	Spec   TranslationSpec
	Fields []FieldSpec
}

func ImportDefinition(source string, lookup RelationLookup) (EntitySpec, error) {
	return importDefinitionClass(source, lookup, "", "")
}

// ImportClassBasedDefinition imports a class whose effective DAL base kind was
// resolved by the semantic index or the plugin-local ancestry scanner. This
// keeps custom abstract base classes editable without guessing from a filename
// or replacing the user's direct parent class.
func ImportClassBasedDefinition(source, definitionClass string, kind DefinitionKind, lookup RelationLookup) (EntitySpec, error) {
	switch kind {
	case DefinitionEntity, DefinitionMapping:
		return importDefinitionClass(source, lookup, definitionClass, kind)
	case DefinitionExtension:
		return importExtensionClass(source, lookup, definitionClass)
	case DefinitionBulkExtension:
		return importBulkExtensionClass(source, lookup, definitionClass)
	default:
		return EntitySpec{}, fmt.Errorf("unsupported class-based DAL kind %q", kind)
	}
}

func importDefinitionClass(source string, lookup RelationLookup, selectedClass string, selectedKind DefinitionKind) (EntitySpec, error) {
	tree := phpparser.Parse(source)
	if len(tree.Errors) != 0 || tree.Tree == nil || tree.Tree.Root == nil {
		return EntitySpec{}, fmt.Errorf("parse entity definition: PHP source contains syntax errors")
	}
	root := tree.Tree.Root
	namespace := strings.Trim(phpquery.Namespace(root), `\`)
	var definition *phpsyntax.Node
	definitionKind := DefinitionEntity
	for _, class := range phpquery.Classes(root) {
		if selectedClass != "" && strings.EqualFold(qualify(namespace, phpquery.ClassName(class)), strings.Trim(selectedClass, `\`)) {
			definition = class
			definitionKind = selectedKind
			break
		}
		for _, parent := range phpquery.ClassExtends(class) {
			short := ShortClass(parent)
			if short == "EntityDefinition" || short == "MappingEntityDefinition" {
				definition = class
				if short == "MappingEntityDefinition" {
					definitionKind = DefinitionMapping
				}
				break
			}
		}
		if definition != nil {
			break
		}
	}
	if definition == nil {
		return EntitySpec{}, fmt.Errorf("PHP file contains no concrete EntityDefinition")
	}
	className := phpquery.ClassName(definition)
	baseName := strings.TrimSuffix(className, "Definition")
	if baseName == className {
		baseName = strings.TrimSuffix(className, "Extension")
	}
	resolve := importClassResolver(root)
	spec := EntitySpec{
		Mode: "edit", DefinitionKind: definitionKind, Namespace: namespace, ClassName: baseName,
		DefinitionClass: qualify(namespace, className),
		CreateMigration: true,
	}
	if definitionKind == DefinitionEntity {
		spec.EntityClass = qualify(namespace, baseName+"Entity")
		spec.CollectionClass = qualify(namespace, baseName+"Collection")
	}
	if match := importedEntityNamePattern.FindStringSubmatch(definition.Text()); len(match) > 1 {
		spec.EntityName = match[1]
	}
	var defineFields *phpsyntax.Node
	var defineProtections *phpsyntax.Node
	for _, method := range phpquery.Methods(definition) {
		switch phpquery.MethodName(method) {
		case "getEntityName":
			if spec.EntityName == "" {
				if literal, ok := importedLiteralStringReturn(method); ok {
					spec.EntityName = literal
				}
			}
		case "getEntityClass":
			if class := returnedImportedClass(method, resolve); class != "" {
				spec.EntityClass = class
			}
		case "getCollectionClass":
			if class := returnedImportedClass(method, resolve); class != "" {
				spec.CollectionClass = class
			}
		case "isInheritanceAware":
			if value, literal := importedBooleanReturn(method); literal {
				spec.InheritanceAware = value
			}
		case "defineFields":
			defineFields = method
		case "defineProtections":
			defineProtections = method
		}
	}
	spec.DefinitionBehavior = importDefinitionBehavior(phpquery.Methods(definition), resolve, lookup, false)
	spec.DefinitionMetadata = importDefinitionMetadata(
		phpquery.Methods(definition), resolve,
		true,
		definitionKind == DefinitionEntity,
		definitionKind == DefinitionEntity,
	)
	if spec.EntityName == "" {
		return EntitySpec{}, fmt.Errorf("entity definition has no literal entity name")
	}
	if defineFields == nil {
		return EntitySpec{}, fmt.Errorf("entity definition has no defineFields method")
	}
	fields, translation, err := importFields(defineFields, resolve, lookup)
	if err != nil {
		return EntitySpec{}, err
	}
	spec.Fields = fields
	importEntityProtections(&spec, defineProtections)
	if translation != nil {
		translation.ParentDefinitionClass = spec.DefinitionClass
		spec.Translation = translation
	}
	return CompleteSpec(spec), nil
}

func importEntityProtections(spec *EntitySpec, method *phpsyntax.Node) {
	if method == nil {
		return
	}
	collections := phpquery.ObjectCreations(method, "EntityProtectionCollection")
	if len(collections) != 1 {
		spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
		return
	}
	arrays := phpquery.Arrays(collections[0])
	if len(arrays) != 1 {
		spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
		return
	}
	for _, item := range phpquery.ArrayItems(arrays[0]) {
		value := phpquery.ArrayItemValue(item)
		if value == nil {
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(item.Text()))
			continue
		}
		creations := phpquery.ObjectCreations(value)
		if len(creations) != 1 {
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(item.Text()))
			continue
		}
		creation := creations[0]
		scopes, recognized := importedProtectionScopes(creation)
		if !recognized {
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(item.Text()))
			continue
		}
		switch ShortClass(phpquery.ObjectClassName(creation)) {
		case "ReadProtection":
			spec.ReadProtected = true
			spec.ReadProtectionScopes = appendUniqueStrings(spec.ReadProtectionScopes, scopes...)
		case "WriteProtection":
			spec.WriteProtected = true
			spec.WriteProtectionScopes = appendUniqueStrings(spec.WriteProtectionScopes, scopes...)
		default:
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(item.Text()))
		}
	}
}

func importedProtectionScopes(creation *phpsyntax.Node) ([]string, bool) {
	arguments := phpquery.Arguments(creation)
	scopes := make([]string, 0, len(arguments))
	for index, argument := range arguments {
		if phpquery.ArgumentName(argument) != "" {
			return nil, false
		}
		expression := phpquery.ArgumentExpression(creation, index)
		if expression != nil && expression.Kind() == phpsyntax.PhpString {
			scopes = append(scopes, phpquery.StringValue(expression))
			continue
		}
		scope, recognized := writeScopeConstant(phpquery.ArgumentValueText(creation, index))
		if !recognized {
			return nil, false
		}
		scopes = append(scopes, scope)
	}
	return scopes, true
}

// ImportExtension imports literal $collection->add(...) calls from an
// EntityExtension. The calls are routed through the same field importer as
// EntityDefinition so extension editing never grows a second DAL field model.
func importedEnumCase(creation *phpsyntax.Node, resolve func(string) string) (string, string) {
	expression := phpquery.ArgumentExpression(creation, 2)
	if expression == nil {
		return "", ""
	}
	text := strings.TrimSpace(expression.Text())
	separator := strings.LastIndex(text, "::")
	if separator <= 0 {
		return "", ""
	}
	className := strings.TrimSpace(text[:separator])
	caseName := strings.TrimSpace(text[separator+2:])
	if className == "" || !propertyPattern.MatchString(caseName) {
		return "", ""
	}
	return resolve(className), caseName
}

// importedFieldCollectionArray supports both new FieldCollection([...]) and
// the common, equivalent local form `$fields = [...]; return new
// FieldCollection($fields);`. It deliberately refuses mutation between the
// literal assignment and constructor so a dynamic collection is never
// misrepresented as a complete static schema.
func importedFieldCollectionArray(method, collection *phpsyntax.Node) *phpsyntax.Node {
	if arrays := phpquery.Arrays(collection); len(arrays) != 0 {
		return arrays[0]
	}
	variable := directVariableExpression(phpquery.ArgumentExpression(collection, 0))
	if variable == "" {
		return nil
	}
	var result *phpsyntax.Node
	for _, statement := range phpquery.ExpressionStatements(method) {
		if statement.Range().Start >= collection.Range().Start {
			break
		}
		if phpquery.AssignedVariable(statement) != variable {
			continue
		}
		value := phpquery.AssignmentValue(statement)
		if value == nil || value.Kind() != phpsyntax.PhpArray {
			result = nil
			continue
		}
		result = value
	}
	if result == nil {
		return nil
	}
	// Appends, element assignments, and mutating method calls would mean the
	// literal is only a partial view of the collection.
	between := method.Text()[result.Range().End-method.Range().Start : collection.Range().Start-method.Range().Start]
	if strings.Contains(between, variable+"[") || strings.Contains(between, variable+"->") {
		return nil
	}
	return result
}

func importConditionalAssociationValue(
	id, raw string,
	value *phpsyntax.Node,
	flags importedFlagSet,
	modifiers importedModifierSet,
	resolve func(string) string,
	lookup RelationLookup,
) (FieldSpec, bool) {
	ternaries := phpquery.Nodes(value, phpsyntax.PhpTernaryExpression)
	if len(ternaries) != 1 {
		return FieldSpec{}, false
	}
	creations := phpquery.ObjectCreations(ternaries[0])
	if len(creations) != 2 {
		return FieldSpec{}, false
	}
	first, firstOK := importedStandaloneAssociation(creations[0], resolve)
	second, secondOK := importedStandaloneAssociation(creations[1], resolve)
	if !firstOK || !secondOK || !sameImportedAssociationIdentity(first, second) {
		return FieldSpec{}, false
	}
	parts := directNodeChildren(ternaries[0])
	if len(parts) < 3 {
		return FieldSpec{}, false
	}
	first.ID = id
	first.Raw = raw
	first.ConditionalAssociation = &ConditionalAssociation{
		ConditionExpression: normalizeImportedPHPExpression(parts[0].Text(), resolve),
		AlternativeKind:     second.Kind, AlternativeAutoload: second.AssociationAutoload,
	}
	first.AssociationAPIAware = flags.apiAware
	first.AssociationAPIAwareSources = append([]string(nil), flags.apiAwareSources...)
	first.AssociationSearchRank = flags.ranking
	first.AssociationSearchTokenize = flags.rankingTokenize
	first.AssociationBehavior = flags.behavior
	first.AssociationMetadata = flags.metadata
	first.AssociationInherited = flags.inherited
	first.AssociationInheritedFK = flags.inheritedForeignKey
	first.ReverseInheritedProperty = flags.reverseInheritedProperty
	first.AssociationFlags = append([]string(nil), flags.preserved...)
	first = withAssociationModifiers(first, modifiers)
	if target, found := lookupRelation(lookup, first.TargetDefinitionClass); found {
		enrichRelation(&first, target)
	}
	return first, true
}

func directVariableExpression(node *phpsyntax.Node) string {
	if node == nil || node.Kind() != phpsyntax.PhpVariable {
		return ""
	}
	if name := phpquery.VariableName(node); name != "" {
		return "$" + name
	}
	return ""
}

// importedLocalFields resolves the narrow, deterministic pattern used by
// version-gated Shopware association declarations: a local variable assigned
// a ternary whose two branches are association constructors, followed by
// addFlags()/description modifiers. It intentionally refuses branches that do
// not describe the same logical association.
func importedLocalFields(method *phpsyntax.Node, resolve func(string) string, lookup RelationLookup) map[string]FieldSpec {
	result := make(map[string]FieldSpec)
	statements := phpquery.ExpressionStatements(method)
	for index, statement := range statements {
		variable := phpquery.AssignedVariable(statement)
		value := phpquery.AssignmentValue(statement)
		if variable == "" || value == nil || value.Kind() != phpsyntax.PhpTernaryExpression {
			continue
		}
		creations := phpquery.ObjectCreations(value)
		if len(creations) != 2 {
			continue
		}
		first, firstOK := importedStandaloneAssociation(creations[0], resolve)
		second, secondOK := importedStandaloneAssociation(creations[1], resolve)
		if !firstOK || !secondOK || !sameImportedAssociationIdentity(first, second) {
			continue
		}
		// Prefer the first branch: it represents the enabled target-version
		// shape in Shopware Feature::isActive() declarations.
		field := first
		parts := directNodeChildren(value)
		if len(parts) < 3 {
			continue
		}
		field.ConditionalAssociation = &ConditionalAssociation{
			ConditionExpression: normalizeImportedPHPExpression(parts[0].Text(), resolve),
			AlternativeKind:     second.Kind, AlternativeAutoload: second.AssociationAutoload,
		}
		for _, following := range statements[index+1:] {
			if phpquery.AssignedVariable(following) != "" {
				break
			}
			text := strings.TrimSpace(following.Text())
			if !strings.HasPrefix(text, variable+"->") {
				continue
			}
			flagSet := importedVariableFlags(following, resolve)
			field.AssociationAPIAware = flagSet.apiAware
			field.AssociationAPIAwareSources = append([]string(nil), flagSet.apiAwareSources...)
			field.AssociationSearchRank = flagSet.ranking
			field.AssociationSearchTokenize = flagSet.rankingTokenize
			field.AssociationBehavior = flagSet.behavior
			field.AssociationMetadata = flagSet.metadata
			field.AssociationInherited = flagSet.inherited
			field.AssociationInheritedFK = flagSet.inheritedForeignKey
			field.ReverseInheritedProperty = flagSet.reverseInheritedProperty
			field.AssociationFlags = append([]string(nil), flagSet.preserved...)
			field.AssociationBeforeFlags = importedVariableModifiers(following, true)
			field.AssociationAfterFlags = importedVariableModifiers(following, false)
			break
		}
		if target, found := lookupRelation(lookup, field.TargetDefinitionClass); found {
			enrichRelation(&field, target)
		}
		result[variable] = field
	}
	return result
}

func directNodeChildren(node *phpsyntax.Node) []*phpsyntax.Node {
	var result []*phpsyntax.Node
	if node == nil {
		return result
	}
	for child := range node.ChildNodes() {
		result = append(result, child)
	}
	return result
}

func importedStandaloneAssociation(creation *phpsyntax.Node, resolve func(string) string) (FieldSpec, bool) {
	field := FieldSpec{Editable: true, UsesExistingColumn: true, Raw: strings.TrimSpace(creation.Text())}
	switch ShortClass(phpquery.ObjectClassName(creation)) {
	case "OneToOneAssociationField":
		field.Kind = FieldOneToOne
		field.PropertyName = importedStringArgument(creation, 0)
		field.StorageName = importedStringArgument(creation, 1)
		field.ReferenceField = defaultString(importedStringArgument(creation, 2), "id")
		field.ReferenceStorageName = field.ReferenceField
		field.TargetDefinitionClass = resolve(importedClassArgument(creation, 3))
		field.AssociationAutoload = importedAssociationAutoload(creation, true)
	case "ManyToOneAssociationField":
		field.Kind = FieldManyToOne
		field.PropertyName = importedStringArgument(creation, 0)
		field.StorageName = importedStringArgument(creation, 1)
		field.TargetDefinitionClass = resolve(importedClassArgument(creation, 2))
		field.ReferenceField = defaultString(importedStringArgument(creation, 3), "id")
		field.ReferenceStorageName = field.ReferenceField
		field.AssociationAutoload = importedAssociationAutoload(creation, false)
	default:
		return FieldSpec{}, false
	}
	return field, field.PropertyName != "" && field.StorageName != "" && field.TargetDefinitionClass != ""
}

func sameImportedAssociationIdentity(left, right FieldSpec) bool {
	return left.PropertyName == right.PropertyName && left.StorageName == right.StorageName &&
		left.ReferenceField == right.ReferenceField && left.TargetDefinitionClass == right.TargetDefinitionClass
}

func importedVariableFlags(statement *phpsyntax.Node, resolve func(string) string) importedFlagSet {
	var result importedFlagSet
	for _, call := range phpquery.Calls(statement) {
		if call.Kind() != phpsyntax.PhpMemberCall || phpquery.CallMethodName(call) != "addFlags" {
			continue
		}
		for argument := range phpquery.Arguments(call) {
			source := strings.TrimSpace(phpquery.ArgumentValueText(call, argument))
			creation := outerObjectCreation(phpquery.ArgumentExpression(call, argument))
			if creation != nil {
				importFlagCreation(&result, creation, source, resolve)
			} else if source != "" {
				result.preserved = append(result.preserved, source)
			}
		}
	}
	return result
}

func importedVariableModifiers(statement *phpsyntax.Node, before bool) []string {
	calls := phpquery.Calls(statement)
	seenFlags := false
	var result []string
	for _, call := range calls {
		method := phpquery.CallMethodName(call)
		if method == "addFlags" {
			seenFlags = true
			continue
		}
		if seenFlags == before {
			continue
		}
		if suffix := importedCallSuffix(call, method); suffix != "" {
			result = append(result, suffix)
		}
	}
	return result
}

func promoteImportedWriteProtection(fields []FieldSpec, translation *TranslationSpec) {
	for index := range fields {
		field := &fields[index]
		promoteWriteProtectedFlags(&field.WriteProtected, &field.WriteProtectedScopes, &field.PreservedFlags)
		promoteWriteProtectedFlags(&field.AssociationWriteProtected, &field.AssociationWriteScopes, &field.AssociationFlags)
		promoteWriteProtectedFlags(&field.TranslationWriteProtected, &field.TranslationWriteScopes, &field.TranslationFlags)
		promoteWriteProtectedFlags(&field.HierarchyChildrenProtected, &field.HierarchyChildrenWriteScopes, &field.HierarchyChildrenFlags)
		promoteWriteProtectedFlags(&field.HierarchyVersionProtected, &field.HierarchyVersionWriteScopes, &field.HierarchyVersionFlags)
	}
	if translation != nil {
		promoteWriteProtectedFlags(
			&translation.AssociationWriteProtected,
			&translation.AssociationWriteScopes,
			&translation.AssociationFlags,
		)
	}
}

func promoteWriteProtectedFlags(enabled *bool, scopes *[]string, values *[]string) {
	remaining := make([]string, 0, len(*values))
	for _, value := range *values {
		parsedScopes, recognized := parseWriteProtectedFlag(value)
		if !recognized {
			remaining = append(remaining, value)
			continue
		}
		*enabled = true
		*scopes = appendUniqueStrings(*scopes, parsedScopes...)
	}
	*values = remaining
}

func parseWriteProtectedFlag(source string) ([]string, bool) {
	parsed := phpparser.Parse("<?php " + source + ";")
	if len(parsed.Errors) != 0 || parsed.Tree == nil || parsed.Tree.Root == nil {
		return nil, false
	}
	creations := phpquery.ObjectCreations(parsed.Tree.Root)
	if len(creations) != 1 || ShortClass(phpquery.ObjectClassName(creations[0])) != "WriteProtected" {
		return nil, false
	}
	creation := creations[0]
	arguments := phpquery.Arguments(creation)
	scopes := make([]string, 0, len(arguments))
	for index, argument := range arguments {
		expression := phpquery.ArgumentExpression(creation, index)
		if expression != nil && expression.Kind() == phpsyntax.PhpString {
			scopes = append(scopes, phpquery.StringValue(expression))
			continue
		}
		scope, recognized := writeScopeConstant(phpquery.ArgumentValueText(creation, index))
		if !recognized || phpquery.ArgumentName(argument) != "" {
			return nil, false
		}
		scopes = append(scopes, scope)
	}
	return scopes, true
}

func writeScopeConstant(expression string) (string, bool) {
	normalized := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(expression), " ", ""), `\`)
	switch normalized {
	case "Context::SYSTEM_SCOPE", `Shopware\Core\Framework\Context::SYSTEM_SCOPE`:
		return "system", true
	case "Context::USER_SCOPE", `Shopware\Core\Framework\Context::USER_SCOPE`:
		return "user", true
	case "Context::CRUD_API_SCOPE", `Shopware\Core\Framework\Context::CRUD_API_SCOPE`:
		return "crud", true
	case "Context::SYSTEM_SCOPE_DAL_WRITE_EVENT", `Shopware\Core\Framework\Context::SYSTEM_SCOPE_DAL_WRITE_EVENT`:
		return "system-scope-dal-write-event", true
	default:
		return "", false
	}
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		if _, found := seen[value]; found {
			continue
		}
		values = append(values, value)
		seen[value] = struct{}{}
	}
	return values
}

func importReferenceVersionField(
	id, raw string,
	creation *phpsyntax.Node,
	flags importedFlagSet,
	modifiers importedModifierSet,
	resolve func(string) string,
	lookup RelationLookup,
) FieldSpec {
	storage := importedStringArgument(creation, 1)
	targetClass := resolve(importedClassArgument(creation, 0))
	field := FieldSpec{
		ID: id, Kind: FieldReferenceVersion,
		StorageName:           storage,
		PropertyName:          camelizeStorageName(storage),
		TargetDefinitionClass: targetClass,
		Required:              flags.required,
		Primary:               flags.primary,
		APIAware:              flags.apiAware,
		APIAwareSources:       append([]string(nil), flags.apiAwareSources...),
		SearchRanking:         flags.ranking,
		SearchRankingTokenize: flags.rankingTokenize,
		Behavior:              flags.behavior,
		Metadata:              flags.metadata,
		Inherited:             flags.inherited,
		InheritedForeignKey:   flags.inheritedForeignKey,
		PreservedFlags:        flags.preserved,
		Editable:              true,
		Raw:                   raw,
	}
	if target, found := lookupRelation(lookup, targetClass); found {
		enrichRelation(&field, target)
		if field.StorageName == "" {
			field.StorageName = target.EntityName + "_version_id"
			field.PropertyName = camelizeStorageName(field.StorageName)
		}
	}
	return withFieldModifiers(field, modifiers)
}

func importedScalarKind(name string) FieldKind {
	switch name {
	case "StringField":
		return FieldString
	case "LongTextField":
		return FieldLongText
	case "IntField":
		return FieldInt
	case "FloatField":
		return FieldFloat
	case "BoolField":
		return FieldBool
	case "DateField":
		return FieldDate
	case "DateTimeField":
		return FieldDateTime
	case "JsonField":
		return FieldJSON
	case "ListField":
		return FieldList
	case "ObjectField":
		return FieldObject
	case "BlobField":
		return FieldBlob
	default:
		return FieldLocked
	}
}

func camelizeStorageName(value string) string {
	parts := strings.Split(value, "_")
	if len(parts) == 0 {
		return value
	}
	for index := 1; index < len(parts); index++ {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, "")
}

func importedDeleteBehavior(creations []*phpsyntax.Node) DeleteBehavior {
	for _, creation := range creations[1:] {
		switch ShortClass(phpquery.ObjectClassName(creation)) {
		case "CascadeDelete":
			return DeleteCascade
		case "SetNullOnDelete":
			return DeleteSetNull
		case "RestrictDelete":
			return DeleteRestrict
		}
	}
	return ""
}

func applyImportedDeleteOptions(field *FieldSpec, creations []*phpsyntax.Node) {
	for _, creation := range creations[1:] {
		arguments := phpquery.Arguments(creation)
		if len(arguments) == 0 {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(phpquery.ArgumentValueText(creation, 0)))
		if value != "true" && value != "false" {
			continue
		}
		option := value == "true"
		switch ShortClass(phpquery.ObjectClassName(creation)) {
		case "CascadeDelete":
			field.DeleteCloneRelevant = &option
		case "SetNullOnDelete":
			field.DeleteEnforcedByConstraint = &option
		}
	}
}

func importedStringArgument(creation *phpsyntax.Node, index int) string {
	return phpquery.StringValue(phpquery.StringArgument(creation, index))
}

func importedClassArgument(creation *phpsyntax.Node, index int) string {
	arguments := phpquery.Arguments(creation)
	if index < 0 || index >= len(arguments) {
		return ""
	}
	return phpquery.ClassConstantName(arguments[index])
}

func importedIntArgument(creation *phpsyntax.Node, index, fallback int) int {
	arguments := phpquery.Arguments(creation)
	if index < 0 || index >= len(arguments) {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(arguments[index].Text()))
	if err != nil {
		return fallback
	}
	return value
}

func importedBoolArgument(creation *phpsyntax.Node, index int) bool {
	arguments := phpquery.Arguments(creation)
	return index >= 0 && index < len(arguments) && strings.EqualFold(strings.TrimSpace(arguments[index].Text()), "true")
}

func importedAssociationAutoload(creation *phpsyntax.Node, fallback bool) bool {
	arguments := phpquery.Arguments(creation)
	for index, argument := range arguments {
		if strings.EqualFold(phpquery.ArgumentName(argument), "autoload") {
			return strings.EqualFold(phpquery.ArgumentValueText(creation, index), "true")
		}
	}
	if len(arguments) <= 4 {
		return fallback
	}
	return strings.EqualFold(phpquery.ArgumentValueText(creation, 4), "true")
}

func importedOptionalIntArgument(creation *phpsyntax.Node, index int) *int {
	arguments := phpquery.Arguments(creation)
	if index < 0 || index >= len(arguments) || strings.EqualFold(strings.TrimSpace(arguments[index].Text()), "null") {
		return nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(arguments[index].Text()))
	if err != nil {
		return nil
	}
	return &value
}

func returnedImportedClass(method *phpsyntax.Node, resolve func(string) string) string {
	returned := singleReturnOnly(method)
	if returned == nil {
		return ""
	}
	expression := returnedExpressionText(returned)
	for _, access := range phpquery.Nodes(returned, phpsyntax.PhpScopedAccess, phpsyntax.PhpMemberAccess) {
		if strings.TrimSpace(access.Text()) != expression {
			continue
		}
		if className := phpquery.ClassConstantName(access); className != "" {
			return resolve(className)
		}
	}
	return ""
}

func singleReturnOnly(method *phpsyntax.Node) *phpsyntax.Node {
	if method == nil {
		return nil
	}
	returns := phpquery.Nodes(method, phpsyntax.PhpReturnStatement)
	if len(returns) != 1 {
		return nil
	}
	text := method.Text()
	open := strings.IndexByte(text, '{')
	close := strings.LastIndexByte(text, '}')
	if open < 0 || close <= open || strings.TrimSpace(text[open+1:close]) != strings.TrimSpace(returns[0].Text()) {
		return nil
	}
	return returns[0]
}

func returnedExpressionText(returned *phpsyntax.Node) string {
	if returned == nil {
		return ""
	}
	statement := strings.TrimSpace(returned.Text())
	if len(statement) < len("return") || !strings.EqualFold(statement[:len("return")], "return") {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(statement[len("return"):]), ";"))
}

func importClassResolver(root *phpsyntax.Node) func(string) string {
	namespace := strings.Trim(phpquery.Namespace(root), `\`)
	currentClass := ""
	if classes := phpquery.Classes(root); len(classes) != 0 {
		currentClass = qualify(namespace, phpquery.ClassName(classes[0]))
	}
	aliases := make(map[string]string)
	for _, declaration := range phpquery.UseDeclarations(root) {
		for _, imported := range phpresolver.ParseUseDeclaration(declaration.Text()) {
			if imported.Kind == phpresolver.ClassImport {
				aliases[strings.ToLower(imported.Alias)] = strings.Trim(imported.Target, `\`)
			}
		}
	}
	return func(name string) string {
		name = strings.Trim(strings.TrimSpace(name), `\`)
		if name == "" {
			return ""
		}
		if strings.EqualFold(name, "self") || strings.EqualFold(name, "static") {
			return currentClass
		}
		parts := strings.SplitN(name, `\`, 2)
		if target, found := aliases[strings.ToLower(parts[0])]; found {
			if len(parts) > 1 {
				return target + `\` + parts[1]
			}
			return target
		}
		if namespace != "" {
			return namespace + `\` + name
		}
		return name
	}
}

func lookupRelation(lookup RelationLookup, class string) (RelationTarget, bool) {
	if lookup != nil {
		if target, found := lookup(class); found {
			return target, true
		}
	}
	class = strings.Trim(class, `\`)
	short := ShortClass(class)
	if !strings.HasSuffix(short, "Definition") {
		return RelationTarget{}, false
	}
	base := strings.TrimSuffix(short, "Definition")
	namespace := strings.TrimSuffix(class, short)
	entityName := importedAcronymBoundary.ReplaceAllString(base, `${1}_${2}`)
	entityName = importedWordBoundary.ReplaceAllString(entityName, `${1}_${2}`)
	return RelationTarget{
		DefinitionClass: class,
		EntityClass:     namespace + base + "Entity",
		CollectionClass: namespace + base + "Collection",
		EntityName:      strings.ToLower(entityName),
		Fields:          []RelationTargetField{{PropertyName: "id", StorageName: "id", Primary: true}},
	}, true
}

func lookupRelationByEntityName(lookup RelationLookup, entityName string) (RelationTarget, bool) {
	if lookup == nil || entityName == "" {
		return RelationTarget{}, false
	}
	target, found := lookup(entityName)
	if !found || target.DefinitionClass == "" || target.EntityName != entityName {
		return RelationTarget{}, false
	}
	return target, true
}

func enrichRelation(field *FieldSpec, target RelationTarget) {
	field.TargetEntityClass = target.EntityClass
	field.TargetCollectionClass = target.CollectionClass
	field.TargetEntityName = target.EntityName
	if field.Kind == FieldOneToMany {
		return
	}
	if field.ReferenceField == "" {
		field.ReferenceField = "id"
	}
	if field.ReferenceStorageName == "" {
		field.ReferenceStorageName = "id"
	}
	for _, candidate := range target.Fields {
		if candidate.PropertyName == field.ReferenceField || candidate.Primary {
			field.ReferenceStorageName = candidate.StorageName
			if candidate.PropertyName == field.ReferenceField {
				break
			}
		}
	}
}

func lockedField(index int, raw string) FieldSpec {
	return FieldSpec{ID: fmt.Sprintf("locked-%d", index+1), Kind: FieldLocked, Editable: false, Raw: strings.TrimSpace(raw)}
}
