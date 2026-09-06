package entityschema

import (
	"fmt"
	"strings"

	phpparser "github.com/shopware/shopware-lsp/internal/parser/php"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

func ImportExtension(source string, lookup RelationLookup) (EntitySpec, error) {
	return importExtensionClass(source, lookup, "")
}

func importExtensionClass(source string, lookup RelationLookup, selectedClass string) (EntitySpec, error) {
	tree := phpparser.Parse(source)
	if len(tree.Errors) != 0 || tree.Tree == nil || tree.Tree.Root == nil {
		return EntitySpec{}, fmt.Errorf("parse entity extension: PHP source contains syntax errors")
	}
	root := tree.Tree.Root
	resolve := importClassResolver(root)
	namespace := strings.Trim(phpquery.Namespace(root), `\`)
	var extension *phpsyntax.Node
	for _, class := range phpquery.Classes(root) {
		if selectedClass != "" && strings.EqualFold(qualify(namespace, phpquery.ClassName(class)), strings.Trim(selectedClass, `\`)) {
			extension = class
			break
		}
		for _, parent := range phpquery.ClassExtends(class) {
			if ShortClass(parent) == "EntityExtension" {
				extension = class
				break
			}
		}
		if extension != nil {
			break
		}
	}
	if extension == nil {
		return EntitySpec{}, fmt.Errorf("PHP file contains no concrete EntityExtension")
	}
	className := phpquery.ClassName(extension)
	baseName := strings.TrimSuffix(className, "Extension")
	if baseName == className {
		baseName = strings.TrimSuffix(className, "Definition")
	}
	spec := EntitySpec{
		Mode: "edit", DefinitionKind: DefinitionExtension,
		Namespace: namespace, ClassName: baseName,
		DefinitionClass: qualify(namespace, className),
		CreateMigration: true,
	}
	var extendFields *phpsyntax.Node
	var extendProtections *phpsyntax.Node
	var modifyFields *phpsyntax.Node
	for _, method := range phpquery.Methods(extension) {
		switch phpquery.MethodName(method) {
		case "extendFields":
			extendFields = method
		case "extendProtections":
			extendProtections = method
		case "modifyFields":
			modifyFields = method
		case "getDefinitionClass":
			if target := returnedImportedClass(method, resolve); target != "" {
				spec.ExtendedDefinitionClass = target
			}
		case "getEntityName":
			if target := returnedEntityNameDefinitionClass(method, resolve); target != "" {
				spec.ExtendedDefinitionClass = target
			}
			if spec.EntityName == "" {
				if literal, ok := importedLiteralStringReturn(method); ok {
					spec.EntityName = literal
				}
			}
		}
	}
	if spec.ExtendedDefinitionClass == "" && spec.EntityName != "" {
		if target, found := lookupRelationByEntityName(lookup, spec.EntityName); found {
			spec.ExtendedDefinitionClass = target.DefinitionClass
			spec.EntityName = target.EntityName
			spec.ExtendedFields = append([]RelationTargetField(nil), target.Fields...)
		}
	}
	if spec.ExtendedDefinitionClass == "" {
		return EntitySpec{}, fmt.Errorf("entity extension target %q is not present in the indexed DAL catalog", spec.EntityName)
	}
	if target, found := lookupRelation(lookup, spec.ExtendedDefinitionClass); found {
		spec.EntityName = target.EntityName
		spec.ExtendedFields = append([]RelationTargetField(nil), target.Fields...)
	}
	if spec.EntityName == "" {
		return EntitySpec{}, fmt.Errorf("entity extension target has no technical entity name")
	}
	var fields []FieldSpec
	var expressions []string
	var err error
	if extendFields != nil {
		fields, expressions, err = importExtensionFields(root, extendFields, lookup)
		if err != nil {
			return EntitySpec{}, err
		}
	}
	spec.Fields = fields
	importEntityExtensionProtections(&spec, extendProtections)
	importEntityExtensionFieldModifications(&spec, modifyFields, resolve)
	spec = CompleteSpec(spec)
	if len(ValidateSpec(spec)) != 0 {
		// Static extension calls can still contain constants, helper calls, or
		// other values the typed field model cannot prove. Preserve every call
		// losslessly instead of exposing a partly editable, invalid spec.
		spec.Fields = make([]FieldSpec, 0, len(expressions))
		for index, expression := range expressions {
			spec.Fields = append(spec.Fields, lockedField(index, expression))
		}
		spec = CompleteSpec(spec)
	}
	return spec, nil
}

// ImportBulkExtension imports a literal BulkEntityExtension::collect method.
// Each yield is represented as one independently validated extension target
// while reusing the normal DAL field importer.
func ImportBulkExtension(source string, lookup RelationLookup) (EntitySpec, error) {
	return importBulkExtensionClass(source, lookup, "")
}

func importBulkExtensionClass(source string, lookup RelationLookup, selectedClass string) (EntitySpec, error) {
	tree := phpparser.Parse(source)
	if len(tree.Errors) != 0 || tree.Tree == nil || tree.Tree.Root == nil {
		return EntitySpec{}, fmt.Errorf("parse bulk entity extension: PHP source contains syntax errors")
	}
	root := tree.Tree.Root
	resolve := importClassResolver(root)
	namespace := strings.Trim(phpquery.Namespace(root), `\`)
	var extension *phpsyntax.Node
	for _, class := range phpquery.Classes(root) {
		if selectedClass != "" && strings.EqualFold(qualify(namespace, phpquery.ClassName(class)), strings.Trim(selectedClass, `\`)) {
			extension = class
			break
		}
		for _, parent := range phpquery.ClassExtends(class) {
			if ShortClass(parent) == "BulkEntityExtension" {
				extension = class
				break
			}
		}
		if extension != nil {
			break
		}
	}
	if extension == nil {
		return EntitySpec{}, fmt.Errorf("PHP file contains no concrete BulkEntityExtension")
	}
	className := phpquery.ClassName(extension)
	baseName := strings.TrimSuffix(className, "BulkEntityExtension")
	if baseName == className {
		baseName = strings.TrimSuffix(className, "BulkExtension")
	}
	if baseName == className {
		baseName = strings.TrimSuffix(className, "Extension")
	}
	if baseName == className {
		baseName = strings.TrimSuffix(className, "Definition")
	}
	spec := EntitySpec{
		Mode: "edit", DefinitionKind: DefinitionBulkExtension,
		Namespace: namespace, ClassName: baseName,
		DefinitionClass: qualify(namespace, className),
		CreateMigration: true,
	}
	var collect *phpsyntax.Node
	for _, method := range phpquery.Methods(extension) {
		if phpquery.MethodName(method) == "collect" {
			collect = method
			break
		}
	}
	if collect == nil {
		return EntitySpec{}, fmt.Errorf("bulk entity extension has no collect method")
	}
	preserveCollect := func() EntitySpec {
		spec.BulkExtensions = nil
		spec.CollectMethodRaw = strings.TrimSpace(collect.Text())
		return CompleteSpec(spec)
	}
	yields := phpquery.Nodes(collect, phpsyntax.PhpYieldExpression)
	if len(yields) == 0 || len(phpquery.Nodes(collect, phpsyntax.PhpExpressionStatement)) != len(yields) {
		return preserveCollect(), nil
	}
	for index, yield := range yields {
		key, value := importedYieldParts(yield)
		if key == nil || value == nil || value.Kind() != phpsyntax.PhpArray {
			return preserveCollect(), nil
		}
		target := BulkExtensionTargetSpec{ID: fmt.Sprintf("bulk-target-%d", index)}
		if key.Kind() == phpsyntax.PhpString {
			target.EntityName = phpquery.StringValue(key)
			if relation, found := lookupRelationByEntityName(lookup, target.EntityName); found {
				target.ExtendedDefinitionClass = relation.DefinitionClass
				target.ExtendedFields = append([]RelationTargetField(nil), relation.Fields...)
			}
		} else if className := phpquery.ScopedAccessClass(key, "ENTITY_NAME"); className != "" {
			target.ExtendedDefinitionClass = resolve(className)
			if relation, found := lookupRelation(lookup, target.ExtendedDefinitionClass); found {
				target.EntityName = relation.EntityName
				target.ExtendedFields = append([]RelationTargetField(nil), relation.Fields...)
			}
		}
		if !entityNamePattern.MatchString(target.EntityName) {
			return preserveCollect(), nil
		}
		var expressions []string
		for _, item := range phpquery.ArrayItems(value) {
			expression := phpquery.ArrayItemValue(item)
			if expression == nil {
				return preserveCollect(), nil
			}
			expressions = append(expressions, strings.TrimSpace(expression.Text()))
		}
		fields, err := importExtensionFieldExpressions(root, expressions, lookup)
		if err != nil {
			return preserveCollect(), nil
		}
		target.Fields = fields
		targetSpec := bulkTargetEntitySpec(spec, target)
		if targetSpec.ExtendedDefinitionClass == "" {
			targetSpec.ExtendedDefinitionClass = spec.DefinitionClass
		}
		if len(ValidateSpec(targetSpec)) != 0 {
			target.Fields = make([]FieldSpec, 0, len(expressions))
			for expressionIndex, expression := range expressions {
				target.Fields = append(target.Fields, lockedField(expressionIndex, expression))
			}
		}
		spec.BulkExtensions = append(spec.BulkExtensions, target)
	}
	return CompleteSpec(spec), nil
}

func importedYieldParts(yield *phpsyntax.Node) (*phpsyntax.Node, *phpsyntax.Node) {
	var key, value *phpsyntax.Node
	afterArrow := false
	for index := 0; index < yield.ChildCount(); index++ {
		switch child := yield.Child(index).(type) {
		case *phpsyntax.Token:
			if child.Kind() == phpsyntax.TkArrow {
				afterArrow = true
			}
		case *phpsyntax.Node:
			if !afterArrow {
				key = child
			} else if value == nil {
				value = child
			}
		}
	}
	return key, value
}

func importEntityExtensionFieldModifications(spec *EntitySpec, method *phpsyntax.Node, resolve func(string) string) {
	if method == nil {
		return
	}
	lock := func() {
		spec.FieldModifications = nil
		spec.ModifyFieldsMethodRaw = strings.TrimSpace(method.Text())
	}
	if strings.Contains(method.Text(), "//") || strings.Contains(method.Text(), "/*") || strings.Contains(method.Text(), "#[") {
		lock()
		return
	}
	byProperty := make(map[string]int)
	var mutationCalls int
	for _, call := range phpquery.Calls(method) {
		operation := phpquery.CallMethodName(call)
		if operation == "get" {
			continue
		}
		if operation != "addFlags" && operation != "removeFlag" {
			lock()
			return
		}
		property := modifiedFieldProperty(call)
		if property == "" {
			lock()
			return
		}
		index, found := byProperty[property]
		if !found {
			index = len(spec.FieldModifications)
			byProperty[property] = index
			spec.FieldModifications = append(spec.FieldModifications, FieldModificationSpec{
				ID: "modify-" + property, PropertyName: property,
			})
		}
		modification := &spec.FieldModifications[index]
		switch operation {
		case "addFlags":
			for argumentIndex := range phpquery.Arguments(call) {
				expression := phpquery.ArgumentExpression(call, argumentIndex)
				creation := outerObjectCreation(expression)
				if creation == nil || strings.TrimSpace(creation.Text()) != strings.TrimSpace(expression.Text()) {
					lock()
					return
				}
				flag, recognized := importedFieldModificationFlag(creation, resolve)
				if !recognized {
					lock()
					return
				}
				modification.AddFlags = append(modification.AddFlags, flag)
			}
		case "removeFlag":
			className := importedClassArgument(call, 0)
			if className == "" {
				lock()
				return
			}
			kind, recognized := fieldFlagKindForClass(resolve(className))
			if !recognized {
				lock()
				return
			}
			modification.RemoveFlags = append(modification.RemoveFlags, kind)
		}
		mutationCalls++
	}
	if len(phpquery.Nodes(method, phpsyntax.PhpExpressionStatement)) != mutationCalls {
		lock()
	}
}

func modifiedFieldProperty(call *phpsyntax.Node) string {
	receiver := phpquery.CallReceiver(call)
	if receiver == nil {
		return ""
	}
	candidates := phpquery.Calls(receiver)
	if phpquery.CallMethodName(receiver) == "get" {
		candidates = append([]*phpsyntax.Node{receiver}, candidates...)
	}
	for _, candidate := range candidates {
		if phpquery.CallMethodName(candidate) != "get" {
			continue
		}
		collection := phpquery.CallReceiver(candidate)
		if collection == nil || strings.TrimSpace(collection.Text()) != "$collection" {
			continue
		}
		expression := phpquery.ArgumentExpression(candidate, 0)
		if expression != nil && expression.Kind() == phpsyntax.PhpString {
			return phpquery.StringValue(expression)
		}
	}
	return ""
}

func fieldFlagKindForClass(className string) (FieldFlagKind, bool) {
	short := ShortClass(className)
	for kind, candidate := range fieldFlagClasses {
		if ShortClass(candidate) == short {
			return kind, true
		}
	}
	return "", false
}

func importedFieldModificationFlag(creation *phpsyntax.Node, resolve func(string) string) (FieldFlagSpec, bool) {
	kind, recognized := fieldFlagKindForClass(resolve(phpquery.ObjectClassName(creation)))
	if !recognized {
		return FieldFlagSpec{}, false
	}
	flag := FieldFlagSpec{Kind: kind}
	switch kind {
	case FlagAPIAware:
		flag.APISources, recognized = importedAPIAwareSources(creation, resolve)
	case FlagSearchRanking:
		flag.SearchRanking, flag.SearchTokenize, recognized = importedSearchRanking(creation)
	case FlagRuntime:
		flag.RuntimeDependencies, flag.RuntimeDependenciesExpression = importedRuntimeDependencies(creation)
	case FlagInherited:
		flag.InheritedForeignKey = importedStringArgument(creation, 0)
		recognized = len(phpquery.Arguments(creation)) == 0 || flag.InheritedForeignKey != ""
	case FlagReverseInherited:
		flag.ReverseProperty = importedStringArgument(creation, 0)
		recognized = flag.ReverseProperty != "" && len(phpquery.Arguments(creation)) == 1
	case FlagWriteProtected:
		flag.WriteScopes, recognized = parseWriteProtectedFlag(strings.TrimSpace(creation.Text()))
	case FlagAllowHTML:
		value, valid := importedDefaultBoolArgument(creation, 0, true)
		flag.AllowHTMLSanitized, recognized = &value, valid
	case FlagCascadeDelete:
		if len(phpquery.Arguments(creation)) != 0 {
			value, valid := importedDefaultBoolArgument(creation, 0, true)
			flag.CloneRelevant, recognized = &value, valid
		}
	case FlagSetNullOnDelete:
		if len(phpquery.Arguments(creation)) != 0 {
			value, valid := importedDefaultBoolArgument(creation, 0, false)
			flag.EnforcedByConstraint, recognized = &value, valid
		}
	case FlagSince:
		flag.Since = importedStringArgument(creation, 0)
		recognized = flag.Since != "" && len(phpquery.Arguments(creation)) == 1
	case FlagDeprecated:
		flag.Deprecated, recognized = importedDeprecation(creation)
	case FlagRuleAreas:
		flag.RuleAreas, recognized = importedRuleAreas(creation)
	case FlagChoice:
		flag.Choice, recognized = importedChoice(creation)
	default:
		recognized = len(phpquery.Arguments(creation)) == 0
	}
	return flag, recognized
}

func importEntityExtensionProtections(spec *EntitySpec, method *phpsyntax.Node) {
	if method == nil {
		return
	}
	if strings.Contains(method.Text(), "//") || strings.Contains(method.Text(), "/*") || strings.Contains(method.Text(), "#[") {
		spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
		return
	}
	var expressions []*phpsyntax.Node
	for _, call := range phpquery.Calls(method) {
		receiver := phpquery.CallReceiver(call)
		if phpquery.CallMethodName(call) != "add" || receiver == nil || strings.TrimSpace(receiver.Text()) != "$protections" {
			spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
			return
		}
		expression := phpquery.ArgumentExpression(call, 0)
		if expression == nil {
			spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
			return
		}
		expressions = append(expressions, expression)
	}
	if len(phpquery.Nodes(method, phpsyntax.PhpExpressionStatement)) != len(expressions) {
		spec.ProtectionMethodRaw = strings.TrimSpace(method.Text())
		return
	}
	for _, expression := range expressions {
		creations := phpquery.ObjectCreations(expression)
		if len(creations) != 1 || strings.TrimSpace(creations[0].Text()) != strings.TrimSpace(expression.Text()) {
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(expression.Text()))
			continue
		}
		creation := creations[0]
		scopes, recognized := importedProtectionScopes(creation)
		if !recognized {
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(expression.Text()))
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
			spec.PreservedProtections = append(spec.PreservedProtections, strings.TrimSpace(expression.Text()))
		}
	}
}

func importExtensionFields(root, method *phpsyntax.Node, lookup RelationLookup) ([]FieldSpec, []string, error) {
	var expressions []string
	for _, call := range phpquery.Calls(method) {
		if phpquery.CallMethodName(call) != "add" {
			continue
		}
		receiver := phpquery.CallReceiver(call)
		if receiver == nil || strings.TrimSpace(receiver.Text()) != "$collection" {
			continue
		}
		argument := phpquery.Argument(call, 0)
		if argument != nil {
			expressions = append(expressions, strings.TrimSpace(argument.Text()))
		}
	}
	if len(expressions) == 0 {
		if len(phpquery.ObjectCreations(method)) != 0 {
			return nil, nil, fmt.Errorf("entity extension fields are not literal $collection->add calls")
		}
		return nil, nil, nil
	}
	fields, err := importExtensionFieldExpressions(root, expressions, lookup)
	return fields, expressions, err
}

func importExtensionFieldExpressions(root *phpsyntax.Node, expressions []string, lookup RelationLookup) ([]FieldSpec, error) {
	if len(expressions) == 0 {
		return nil, nil
	}
	var synthetic strings.Builder
	synthetic.WriteString("<?php declare(strict_types=1);\nnamespace ")
	synthetic.WriteString(strings.Trim(phpquery.Namespace(root), `\`))
	synthetic.WriteString(";\n")
	for _, declaration := range phpquery.UseDeclarations(root) {
		synthetic.WriteString(strings.TrimSpace(declaration.Text()))
		synthetic.WriteByte('\n')
	}
	synthetic.WriteString("class ShopwareLspImportedDefinition extends \\Shopware\\Core\\Framework\\DataAbstractionLayer\\EntityDefinition {\n")
	synthetic.WriteString("public const ENTITY_NAME = 'shopware_lsp_extension';\n")
	synthetic.WriteString("protected function defineFields(): \\Shopware\\Core\\Framework\\DataAbstractionLayer\\FieldCollection { return new FieldCollection([\n")
	synthetic.WriteString(strings.Join(expressions, ",\n"))
	synthetic.WriteString("\n]); }\n}\n")
	imported, err := ImportDefinition(synthetic.String(), lookup)
	if err != nil {
		return nil, fmt.Errorf("import entity extension fields: %w", err)
	}
	return imported.Fields, nil
}

func returnedEntityNameDefinitionClass(method *phpsyntax.Node, resolve func(string) string) string {
	returned := singleReturnOnly(method)
	if returned == nil {
		return ""
	}
	expression := returnedExpressionText(returned)
	for _, access := range phpquery.Nodes(returned, phpsyntax.PhpScopedAccess, phpsyntax.PhpMemberAccess) {
		if strings.TrimSpace(access.Text()) != expression {
			continue
		}
		if className := phpquery.ScopedAccessClass(access, "ENTITY_NAME"); className != "" {
			return resolve(className)
		}
	}
	return ""
}

func importedBooleanReturn(method *phpsyntax.Node) (bool, bool) {
	returned := singleReturnOnly(method)
	if returned == nil {
		return false, false
	}
	booleans := phpquery.Nodes(returned, phpsyntax.PhpBoolean)
	if len(booleans) != 1 || strings.TrimSpace(booleans[0].Text()) != returnedExpressionText(returned) {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(booleans[0].Text())) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

// ImportTranslationDefinition imports the literal, designer-owned portion of
// an EntityTranslationDefinition. AttachTranslation pairs it with the parent
// definition's TranslatedField facades.
