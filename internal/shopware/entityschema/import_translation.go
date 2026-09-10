package entityschema

import (
	"fmt"
	"strings"

	phpparser "github.com/shopware/shopware-lsp/internal/parser/php"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

func ImportTranslationDefinition(source string, lookup RelationLookup) (ImportedTranslation, error) {
	return importTranslationClass(source, lookup, "")
}

func importTranslationClass(source string, lookup RelationLookup, selectedClass string) (ImportedTranslation, error) {
	tree := phpparser.Parse(source)
	if len(tree.Errors) != 0 || tree.Tree == nil || tree.Tree.Root == nil {
		return ImportedTranslation{}, fmt.Errorf("parse translation definition: PHP source contains syntax errors")
	}
	root := tree.Tree.Root
	namespace := strings.Trim(phpquery.Namespace(root), `\`)
	var definition *phpsyntax.Node
	for _, class := range phpquery.Classes(root) {
		if selectedClass != "" && strings.EqualFold(qualify(namespace, phpquery.ClassName(class)), strings.Trim(selectedClass, `\`)) {
			definition = class
			break
		}
		for _, parent := range phpquery.ClassExtends(class) {
			if ShortClass(parent) == "EntityTranslationDefinition" {
				definition = class
				break
			}
		}
		if definition != nil {
			break
		}
	}
	if definition == nil {
		return ImportedTranslation{}, fmt.Errorf("PHP file contains no EntityTranslationDefinition")
	}
	className := phpquery.ClassName(definition)
	baseName := strings.TrimSuffix(className, "Definition")
	resolve := importClassResolver(root)
	result := ImportedTranslation{Spec: TranslationSpec{
		Enabled: true, DefinitionClass: qualify(namespace, className),
		EntityClass: qualify(namespace, baseName+"Entity"), CollectionClass: qualify(namespace, baseName+"Collection"),
	}}
	if match := importedEntityNamePattern.FindStringSubmatch(definition.Text()); len(match) > 1 {
		result.Spec.EntityName = match[1]
	}
	var defineFields *phpsyntax.Node
	for _, method := range phpquery.Methods(definition) {
		switch phpquery.MethodName(method) {
		case "getEntityName":
			if result.Spec.EntityName == "" {
				if literal, ok := importedLiteralStringReturn(method); ok {
					result.Spec.EntityName = literal
				}
			}
		case "getEntityClass":
			if class := returnedImportedClass(method, resolve); class != "" {
				result.Spec.EntityClass = class
			}
		case "getCollectionClass":
			if class := returnedImportedClass(method, resolve); class != "" {
				result.Spec.CollectionClass = class
			}
		case "getParentDefinitionClass":
			result.Spec.ParentDefinitionClass = returnedImportedClass(method, resolve)
		case "defineFields":
			defineFields = method
		}
	}
	result.Spec.DefinitionMetadata = importDefinitionMetadata(
		phpquery.Methods(definition), resolve, true, false, false,
	)
	result.Spec.DefinitionBehavior = importDefinitionBehavior(phpquery.Methods(definition), resolve, lookup, true)
	if result.Spec.EntityName == "" && result.Spec.ParentDefinitionClass != "" {
		if target, found := lookupRelation(lookup, result.Spec.ParentDefinitionClass); found {
			result.Spec.EntityName = target.EntityName + "_translation"
		} else if parentName := inferEntityName(result.Spec.ParentDefinitionClass); parentName != "" {
			result.Spec.EntityName = parentName + "_translation"
		}
	}
	if result.Spec.EntityName == "" || result.Spec.ParentDefinitionClass == "" || defineFields == nil {
		return ImportedTranslation{}, fmt.Errorf("translation definition has incomplete literal identity or fields")
	}
	fields, _, err := importFields(defineFields, resolve, lookup)
	if err != nil {
		return ImportedTranslation{}, err
	}
	result.Fields = fields
	return result, nil
}

// AttachTranslation replaces parent TranslatedField placeholders with the
// concrete storage fields imported from the companion definition.
func AttachTranslation(parent EntitySpec, imported ImportedTranslation) EntitySpec {
	childByProperty := make(map[string]int)
	for index, field := range imported.Fields {
		if field.Kind != FieldLocked && field.PropertyName != "" {
			childByProperty[field.PropertyName] = index
		}
	}
	used := make(map[int]struct{})
	combined := make([]FieldSpec, 0, len(parent.Fields)+len(imported.Fields))
	for _, parentField := range parent.Fields {
		childIndex, found := childByProperty[parentField.PropertyName]
		if !parentField.Translated || !found {
			combined = append(combined, parentField)
			continue
		}
		field := imported.Fields[childIndex]
		field.ID = parentField.ID
		field.Translated = true
		field.TranslationUseForSort = parentField.TranslationUseForSort
		field.TranslationAPIAware = parentField.TranslationAPIAware
		field.TranslationAPIAwareSources = append([]string(nil), parentField.TranslationAPIAwareSources...)
		field.TranslationSearchRank = parentField.TranslationSearchRank
		field.TranslationSearchTokenize = parentField.TranslationSearchTokenize
		field.TranslationBehavior = parentField.TranslationBehavior
		field.TranslationMetadata = parentField.TranslationMetadata
		field.TranslationWriteProtected = parentField.TranslationWriteProtected
		field.TranslationWriteScopes = append([]string(nil), parentField.TranslationWriteScopes...)
		field.TranslationInherited = parentField.TranslationInherited
		field.TranslationInheritedFK = parentField.TranslationInheritedFK
		field.TranslationFlags = append([]string(nil), parentField.TranslationFlags...)
		field.TranslationBeforeFlags = append([]string(nil), parentField.TranslationBeforeFlags...)
		field.TranslationAfterFlags = append([]string(nil), parentField.TranslationAfterFlags...)
		combined = append(combined, field)
		used[childIndex] = struct{}{}
	}
	for index, field := range imported.Fields {
		if _, found := used[index]; found {
			continue
		}
		field.ID = "translation-" + field.ID
		field.Translated = true
		field.TranslationDefinitionOnly = true
		combined = append(combined, field)
	}
	parent.Fields = combined
	translation := imported.Spec
	if parent.Translation != nil {
		translation.ParentStorageName = parent.Translation.ParentStorageName
		translation.ParentPropertyName = parent.Translation.ParentPropertyName
		translation.AssociationProperty = parent.Translation.AssociationProperty
		translation.AssociationLocalField = parent.Translation.AssociationLocalField
		translation.AssociationRequired = parent.Translation.AssociationRequired
		translation.AssociationAPIAware = parent.Translation.AssociationAPIAware
		translation.AssociationAPIAwareSources = append([]string(nil), parent.Translation.AssociationAPIAwareSources...)
		translation.AssociationBehavior = parent.Translation.AssociationBehavior
		translation.AssociationMetadata = parent.Translation.AssociationMetadata
		translation.AssociationWriteProtected = parent.Translation.AssociationWriteProtected
		translation.AssociationWriteScopes = append([]string(nil), parent.Translation.AssociationWriteScopes...)
		translation.AssociationInherited = parent.Translation.AssociationInherited
		translation.AssociationInheritedFK = parent.Translation.AssociationInheritedFK
		translation.ReverseInheritedProperty = parent.Translation.ReverseInheritedProperty
		translation.AssociationFlags = append([]string(nil), parent.Translation.AssociationFlags...)
		translation.AssociationBeforeFlags = append([]string(nil), parent.Translation.AssociationBeforeFlags...)
		translation.AssociationAfterFlags = append([]string(nil), parent.Translation.AssociationAfterFlags...)
	}
	parent.Translation = &translation
	return CompleteSpec(parent)
}
