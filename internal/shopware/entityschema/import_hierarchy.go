package entityschema

func (r *definitionFieldImporter) hierarchyField(id string) *FieldSpec {
	if r.hierarchyIndex < 0 {
		r.hierarchyIndex = len(r.fields)
		r.fields = append(r.fields, FieldSpec{
			ID: id, Kind: FieldHierarchy, PropertyName: "children", HierarchyParentProperty: "parent", ForeignKeyPropertyName: "parentId",
			StorageName: "parent_id", ReferenceField: "id", ReferenceStorageName: "id", DeleteBehavior: DeleteCascade, Editable: true,
		})
	}
	return &r.fields[r.hierarchyIndex]
}

func (r *definitionFieldImporter) importHierarchyField(e *importedFieldExpression) bool {
	switch e.kindName {
	case "ParentFkField", "ParentAssociationField", "ChildrenAssociationField":
	default:
		return false
	}
	field := r.hierarchyField(e.id)
	switch e.kindName {
	case "ParentFkField":
		field.TargetDefinitionClass = r.resolve(importedClassArgument(e.creation, 0))
		field.Required = false
		field.Primary = false
		field.APIAware = e.flags.apiAware
		field.APIAwareSources = append([]string(nil), e.flags.apiAwareSources...)
		field.SearchRanking = e.flags.ranking
		field.SearchRankingTokenize = e.flags.rankingTokenize
		field.Behavior = e.flags.behavior
		field.Metadata = e.flags.metadata
		field.Inherited = e.flags.inherited
		field.InheritedForeignKey = e.flags.inheritedForeignKey
		field.PreservedFlags = e.flags.preserved
		field.Raw = e.raw
		*field = withFieldModifiers(*field, e.modifiers)
	case "ParentAssociationField":
		field.TargetDefinitionClass = r.resolve(importedClassArgument(e.creation, 0))
		field.ReferenceField = defaultString(importedStringArgument(e.creation, 1), "id")
		field.ReferenceStorageName = field.ReferenceField
		applyImportedAssociationFlags(field, e.flags, e.modifiers)
	case "ChildrenAssociationField":
		field.TargetDefinitionClass = r.resolve(importedClassArgument(e.creation, 0))
		field.PropertyName = defaultString(importedStringArgument(e.creation, 1), "children")
		field.HierarchyChildrenFlags = e.flags.preserved
		field.HierarchyChildrenAPIAware = e.flags.apiAware
		field.HierarchyChildrenAPISources = append([]string(nil), e.flags.apiAwareSources...)
		field.HierarchyChildrenRank = e.flags.ranking
		field.HierarchyChildrenTokenize = e.flags.rankingTokenize
		field.HierarchyChildrenBehavior = e.flags.behavior
		field.HierarchyChildrenMetadata = e.flags.metadata
		field.HierarchyChildrenInherited = e.flags.inherited
		field.HierarchyChildrenInheritedFK = e.flags.inheritedForeignKey
		field.HierarchyChildrenReverse = e.flags.reverseInheritedProperty
		field.HierarchyChildrenBefore = e.modifiers.beforeFlags
		field.HierarchyChildrenAfter = e.modifiers.afterFlags
		field.DeleteBehavior = DeleteCascade
	}
	if target, found := lookupRelation(r.lookup, field.TargetDefinitionClass); found {
		enrichRelation(field, target)
	}
	return true
}

func collapseImportedHierarchy(fields []FieldSpec, hierarchyIndex int) []FieldSpec {
	if hierarchyIndex < 0 {
		return fields
	}
	hierarchy := fields[hierarchyIndex]
	referenceVersionIndex := -1
	for index, field := range fields {
		if index == hierarchyIndex || field.Kind != FieldReferenceVersion ||
			field.StorageName != "parent_version_id" ||
			field.TargetDefinitionClass != hierarchy.TargetDefinitionClass {
			continue
		}
		referenceVersionIndex = index
		hierarchy.HierarchyVersionAware = true
		hierarchy.HierarchyVersionAPIAware = field.APIAware
		hierarchy.HierarchyVersionAPISources = append([]string(nil), field.APIAwareSources...)
		hierarchy.HierarchyVersionBehavior = field.Behavior
		hierarchy.HierarchyVersionMetadata = field.Metadata
		hierarchy.HierarchyVersionInherited = field.Inherited
		hierarchy.HierarchyVersionInheritedFK = field.InheritedForeignKey
		hierarchy.HierarchyVersionFlags = append([]string(nil), field.PreservedFlags...)
		hierarchy.HierarchyVersionBefore = append([]string(nil), field.ModifiersBeforeFlags...)
		hierarchy.HierarchyVersionAfter = append([]string(nil), field.ModifiersAfterFlags...)
		break
	}
	collapsed := make([]FieldSpec, 0, len(fields))
	for index, field := range fields {
		if index == referenceVersionIndex {
			continue
		}
		if index == hierarchyIndex {
			field = hierarchy
		}
		collapsed = append(collapsed, field)
	}
	return collapsed
}
