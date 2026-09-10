package entityschema

import (
	"fmt"
	"strings"

	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

type importedForeignKey struct {
	field FieldSpec
	raw   string
}

// definitionFieldImporter owns the ordered assembly of a single FieldCollection.
// Foreign keys and hierarchy fragments are combined without changing source IDs.
type definitionFieldImporter struct {
	resolve        func(string) string
	lookup         RelationLookup
	fields         []FieldSpec
	translation    *TranslationSpec
	foreignKeys    map[string]importedForeignKey
	localFields    map[string]FieldSpec
	hierarchyIndex int
}

type importedFieldExpression struct {
	index             int
	id, raw, kindName string
	value, creation   *phpsyntax.Node
	creations         []*phpsyntax.Node
	flags             importedFlagSet
	modifiers         importedModifierSet
}

func importFields(method *phpsyntax.Node, resolve func(string) string, lookup RelationLookup) ([]FieldSpec, *TranslationSpec, error) {
	collections := phpquery.ObjectCreations(method, "FieldCollection")
	if len(collections) == 0 {
		return nil, nil, fmt.Errorf("defineFields does not return a literal FieldCollection")
	}
	array := importedFieldCollectionArray(method, collections[0])
	if array == nil {
		return nil, nil, fmt.Errorf("defineFields FieldCollection has no literal array")
	}
	items := phpquery.ArrayItems(array)
	// Each array item contributes at most one field; FK pairing and hierarchy
	// assembly can reduce that count. Reserve once to avoid copying large specs.
	importer := definitionFieldImporter{
		resolve: resolve, lookup: lookup,
		fields:         make([]FieldSpec, 0, len(items)),
		foreignKeys:    make(map[string]importedForeignKey),
		localFields:    importedLocalFields(method, resolve, lookup),
		hierarchyIndex: -1,
	}
	for index, item := range items {
		importer.importItem(index, item)
	}
	fields := collapseImportedHierarchy(importer.fields, importer.hierarchyIndex)
	fields = appendUnassociatedForeignKeys(fields, importer.foreignKeys)
	if len(fields) == 0 {
		fields = nil
	}
	promoteImportedWriteProtection(fields, importer.translation)
	return fields, importer.translation, nil
}

func (r *definitionFieldImporter) importItem(index int, item *phpsyntax.Node) {
	value := phpquery.ArrayItemValue(item)
	if value == nil {
		return
	}
	creations := phpquery.ObjectCreations(value)
	if len(creations) == 0 {
		r.importLocalOrLockedField(index, item, value)
		return
	}
	creation := creations[0]
	expression := importedFieldExpression{
		index: index, id: fmt.Sprintf("field-%d", index+1),
		raw:      strings.TrimSpace(item.Text()),
		kindName: ShortClass(phpquery.ObjectClassName(creation)),
		value:    value, creation: creation, creations: creations,
		flags:     importedFlags(value, creation, r.resolve),
		modifiers: importedModifiers(value, creation),
	}
	if r.importValueField(&expression) || r.importHierarchyField(&expression) {
		return
	}
	if expression.kindName == "FkField" {
		r.importForeignKey(&expression)
		return
	}
	if field, found := r.importAssociationField(&expression); found {
		r.fields = append(r.fields, field)
		return
	}
	r.fields = append(r.fields, lockedField(index, expression.raw))
}

func (r *definitionFieldImporter) importLocalOrLockedField(index int, item, value *phpsyntax.Node) {
	if variable := directVariableExpression(value); variable != "" {
		if local, found := r.localFields[variable]; found {
			local.ID = fmt.Sprintf("field-%d", index+1)
			r.fields = append(r.fields, local)
			return
		}
	}
	r.fields = append(r.fields, lockedField(index, item.Text()))
}

func (r *definitionFieldImporter) importValueField(e *importedFieldExpression) bool {
	if conditional, found := importConditionalAssociationValue(e.id, e.raw, e.value, e.flags, e.modifiers, r.resolve, r.lookup); found {
		r.fields = append(r.fields, conditional)
		return true
	}
	if specialized, found := importSpecializedField(e.id, e.raw, e.creation, e.flags, e.modifiers, r.resolve); found {
		r.fields = append(r.fields, specialized)
		return true
	}
	simple, translation, found := importSimpleDefinitionField(e.index, e.id, e.raw, e.kindName, e.creation, e.flags, e.modifiers, r.resolve, r.lookup)
	if !found {
		return false
	}
	if translation != nil {
		r.translation = translation
	}
	if simple.Kind != "" {
		r.fields = append(r.fields, simple)
	}
	return true
}
