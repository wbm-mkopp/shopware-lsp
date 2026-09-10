package entityschema

import "sort"

func (r *definitionFieldImporter) importForeignKey(e *importedFieldExpression) {
	storage := importedStringArgument(e.creation, 0)
	flags := e.flags
	r.foreignKeys[storage] = importedForeignKey{field: withFieldModifiers(FieldSpec{
		ID: e.id, Kind: FieldManyToOne, StorageName: storage,
		ForeignKeyPropertyName: importedStringArgument(e.creation, 1),
		TargetDefinitionClass:  r.resolve(importedClassArgument(e.creation, 2)),
		ReferenceField:         defaultString(importedStringArgument(e.creation, 3), "id"),
		Required:               flags.required, Primary: flags.primary, APIAware: flags.apiAware,
		APIAwareSources: append([]string(nil), flags.apiAwareSources...),
		SearchRanking:   flags.ranking, SearchRankingTokenize: flags.rankingTokenize, Behavior: flags.behavior, Metadata: flags.metadata,
		Inherited: flags.inherited, InheritedForeignKey: flags.inheritedForeignKey,
		PreservedFlags: flags.preserved, Editable: true,
	}, e.modifiers), raw: e.raw}
}

func (r *definitionFieldImporter) importAssociationField(e *importedFieldExpression) (FieldSpec, bool) {
	switch e.kindName {
	case "ManyToOneAssociationField":
		return r.importToOneAssociation(e, FieldManyToOne), true
	case "OneToOneAssociationField":
		return r.importToOneAssociation(e, FieldOneToOne), true
	case "OneToManyAssociationField":
		field := FieldSpec{
			ID: e.id, Kind: FieldOneToMany, PropertyName: importedStringArgument(e.creation, 0),
			TargetDefinitionClass: r.resolve(importedClassArgument(e.creation, 1)),
			ReferenceStorageName:  importedStringArgument(e.creation, 2),
			SourceColumn:          defaultString(importedStringArgument(e.creation, 3), "id"), Editable: true, Raw: e.raw,
		}
		r.finishAssociation(&field, e)
		return field, true
	case "ManyToManyAssociationField":
		field := FieldSpec{
			ID: e.id, Kind: FieldManyToMany, PropertyName: importedStringArgument(e.creation, 0),
			TargetDefinitionClass:  r.resolve(importedClassArgument(e.creation, 1)),
			MappingDefinitionClass: r.resolve(importedClassArgument(e.creation, 2)),
			MappingLocalColumn:     importedStringArgument(e.creation, 3), MappingReferenceColumn: importedStringArgument(e.creation, 4),
			SourceColumn:   defaultString(importedStringArgument(e.creation, 5), "id"),
			ReferenceField: defaultString(importedStringArgument(e.creation, 6), "id"), Editable: true, Raw: e.raw,
		}
		r.finishAssociation(&field, e)
		return field, true
	default:
		return FieldSpec{}, false
	}
}

// To-one constructors use different argument orders and autoload defaults, but
// share the same foreign-key pairing and association-flag assembly.
func (r *definitionFieldImporter) importToOneAssociation(e *importedFieldExpression, kind FieldKind) FieldSpec {
	classArgument, referenceArgument, autoload := 2, 3, false
	if kind == FieldOneToOne {
		classArgument, referenceArgument, autoload = 3, 2, true
	}
	storage := importedStringArgument(e.creation, 1)
	foreignKey, found := r.foreignKeys[storage]
	field := foreignKey.field
	if !found {
		field = FieldSpec{ID: e.id, StorageName: storage, UsesExistingColumn: true, Editable: true, Raw: e.raw}
	}
	field.Kind = kind
	field.PropertyName = importedStringArgument(e.creation, 0)
	field.TargetDefinitionClass = r.resolve(importedClassArgument(e.creation, classArgument))
	field.ReferenceField = defaultString(importedStringArgument(e.creation, referenceArgument), "id")
	field.ReferenceStorageName = field.ReferenceField
	field.AssociationAutoload = importedAssociationAutoload(e.creation, autoload)
	if found {
		field.Raw = foreignKey.raw + ",\n" + e.raw
		delete(r.foreignKeys, storage)
	}
	r.finishAssociation(&field, e)
	return field
}

func (r *definitionFieldImporter) finishAssociation(field *FieldSpec, e *importedFieldExpression) {
	field.DeleteBehavior = importedDeleteBehavior(e.creations)
	applyImportedDeleteOptions(field, e.creations)
	applyImportedAssociationFlags(field, e.flags, e.modifiers)
	if target, found := lookupRelation(r.lookup, field.TargetDefinitionClass); found {
		enrichRelation(field, target)
	}
}

func applyImportedAssociationFlags(field *FieldSpec, flags importedFlagSet, modifiers importedModifierSet) {
	field.AssociationFlags = flags.preserved
	field.AssociationAPIAware = flags.apiAware
	field.AssociationAPIAwareSources = append([]string(nil), flags.apiAwareSources...)
	field.AssociationSearchRank = flags.ranking
	field.AssociationSearchTokenize = flags.rankingTokenize
	field.AssociationBehavior = flags.behavior
	field.AssociationMetadata = flags.metadata
	field.AssociationInherited = flags.inherited
	field.AssociationInheritedFK = flags.inheritedForeignKey
	field.ReverseInheritedProperty = flags.reverseInheritedProperty
	field.AssociationBeforeFlags = append([]string(nil), modifiers.beforeFlags...)
	field.AssociationAfterFlags = append([]string(nil), modifiers.afterFlags...)
}

func appendUnassociatedForeignKeys(fields []FieldSpec, foreignKeys map[string]importedForeignKey) []FieldSpec {
	foreignKeyNames := make([]string, 0, len(foreignKeys))
	for storage := range foreignKeys {
		foreignKeyNames = append(foreignKeyNames, storage)
	}
	sort.Strings(foreignKeyNames)
	for _, storage := range foreignKeyNames {
		foreignKey := foreignKeys[storage]
		field := foreignKey.field
		field.Kind = FieldForeignKey
		field.PropertyName = field.ForeignKeyPropertyName
		field.ForeignKeyPropertyName = ""
		field.Raw = foreignKey.raw
		fields = append(fields, field)
	}
	return fields
}
