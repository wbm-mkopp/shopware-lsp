package entityschema

import (
	"fmt"
	"strings"
	"testing"

	php "github.com/shopware/shopware-lsp/internal/parser/php"
	"github.com/stretchr/testify/require"
)

func associationImportSource(fields string) string {
	return `<?php
namespace Acme\Example;
use Shopware\Core\Framework\DataAbstractionLayer\EntityDefinition;
use Shopware\Core\Framework\DataAbstractionLayer\FieldCollection;
use Shopware\Core\Framework\DataAbstractionLayer\Field\IdField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\FkField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\ManyToOneAssociationField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\OneToOneAssociationField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\OneToManyAssociationField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\ManyToManyAssociationField;
use Shopware\Core\Framework\DataAbstractionLayer\Field\Flag\Required;
use Shopware\Core\Framework\DataAbstractionLayer\Field\Flag\ApiAware;
use Shopware\Core\Framework\DataAbstractionLayer\Field\Flag\CascadeDelete;
use Shopware\Core\Framework\DataAbstractionLayer\Field\Flag\Inherited;
use Shopware\Core\Framework\DataAbstractionLayer\Field\Flag\CustomFlag;
class ExampleDefinition extends EntityDefinition {
 public const ENTITY_NAME = 'acme_example';
 protected function defineFields(): FieldCollection {
 return new FieldCollection([new IdField('id', 'id'), ` + fields + `]);
 }
}`
}

func associationImportTarget(class string) (RelationTarget, bool) {
	if class != `Acme\Example\TargetDefinition` {
		return RelationTarget{}, false
	}
	return RelationTarget{DefinitionClass: class, EntityName: "target", EntityClass: `Acme\Example\TargetEntity`, CollectionClass: `Acme\Example\TargetCollection`, Fields: []RelationTargetField{{PropertyName: "code", StorageName: "target_code", Primary: true}}}, true
}

func TestImportToOneAssociationsPreserveColumnAndAssociationContracts(t *testing.T) {
	for _, kind := range []FieldKind{FieldManyToOne, FieldOneToOne} {
		for _, withColumn := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/column=%t", kind, withColumn), func(t *testing.T) {
				constructor := "new ManyToOneAssociationField('target', 'target_id', TargetDefinition::class, 'code')"
				if kind == FieldOneToOne {
					constructor = "new OneToOneAssociationField('target', 'target_id', 'code', TargetDefinition::class)"
				}
				column := ""
				if !withColumn {
					constructor = strings.Replace(constructor, "'target_id'", "'id'", 1)
				}
				if withColumn {
					column = "(new FkField('target_id', 'targetId', TargetDefinition::class, 'code'))->addFlags(new Required())->setDescription('Column'),\n"
				}
				source := associationImportSource(column + "(" + constructor + ")->removeFlag(Inherited::class)->addFlags(new ApiAware(), new CascadeDelete(), new CustomFlag())->setDescription('Association')")
				spec, err := ImportDefinition(source, associationImportTarget)
				require.NoError(t, err)
				require.Len(t, spec.Fields, 2)
				field := spec.Fields[1]
				require.Equal(t, kind, field.Kind)
				require.Equal(t, "target", field.PropertyName)
				require.Equal(t, "target_code", field.ReferenceStorageName)
				require.Equal(t, "target", field.TargetEntityName)
				require.Equal(t, withColumn, field.Required)
				require.Equal(t, !withColumn, field.UsesExistingColumn)
				require.Equal(t, kind == FieldOneToOne, field.AssociationAutoload)
				require.False(t, field.APIAware)
				require.True(t, field.AssociationAPIAware)
				require.Equal(t, DeleteCascade, field.DeleteBehavior)
				require.Equal(t, []string{"->removeFlag(Inherited::class)"}, field.AssociationBeforeFlags)
				require.Equal(t, []string{"->setDescription('Association')"}, field.AssociationAfterFlags)
				require.Contains(t, field.AssociationFlags, "new CustomFlag()")
				if withColumn {
					require.Equal(t, "targetId", field.ForeignKeyPropertyName)
					require.Equal(t, []string{"->setDescription('Column')"}, field.ModifiersAfterFlags)
					require.Contains(t, field.Raw, "new FkField")
				} else {
					require.Empty(t, field.ForeignKeyPropertyName)
				}
				rendered, err := RenderDefinition(spec)
				require.NoError(t, err)
				require.Empty(t, php.Parse(rendered).Errors)
				reimported, err := ImportDefinition(rendered, associationImportTarget)
				require.NoError(t, err)
				again := fieldByProperty(t, reimported.Fields, "target")
				require.Equal(t, field.Kind, again.Kind)
				require.Equal(t, field.UsesExistingColumn, again.UsesExistingColumn)
				require.Equal(t, field.Required, again.Required)
				require.Equal(t, field.AssociationAutoload, again.AssociationAutoload)
				require.Equal(t, field.AssociationFlags, again.AssociationFlags)
				require.Equal(t, field.AssociationBeforeFlags, again.AssociationBeforeFlags)
				require.Equal(t, field.AssociationAfterFlags, again.AssociationAfterFlags)
			})
		}
	}
}

func TestImportFieldsPreservesUnknownExpressionsAndOrdersUnpairedColumns(t *testing.T) {
	source := associationImportSource(`new FkField('z_id', 'zId', TargetDefinition::class),
 customFieldFactory(),
 new FkField('a_id', 'aId', TargetDefinition::class),
 new UnknownField('opaque'),`)
	spec, err := ImportDefinition(source, associationImportTarget)
	require.NoError(t, err)
	require.Len(t, spec.Fields, 5)
	require.Equal(t, FieldLocked, spec.Fields[1].Kind)
	require.Equal(t, "customFieldFactory()", spec.Fields[1].Raw)
	require.False(t, spec.Fields[1].Editable)
	require.Equal(t, FieldLocked, spec.Fields[2].Kind)
	require.Equal(t, "new UnknownField('opaque')", spec.Fields[2].Raw)
	require.Equal(t, "a_id", spec.Fields[3].StorageName)
	require.Equal(t, "z_id", spec.Fields[4].StorageName)
	rendered, err := RenderDefinition(spec)
	require.NoError(t, err)
	require.Contains(t, rendered, "customFieldFactory()")
	require.Contains(t, rendered, "new UnknownField('opaque')")
}

func BenchmarkImportDefinitionAssociations(b *testing.B) {
	var fields strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&fields, "new FkField('target_%d_id', 'target%dId', TargetDefinition::class), new ManyToOneAssociationField('target%d', 'target_%d_id', TargetDefinition::class),\n", i, i, i, i)
	}
	source := associationImportSource(fields.String())
	b.ReportAllocs()
	var capacity int
	for b.Loop() {
		spec, err := ImportDefinition(source, associationImportTarget)
		if err != nil || len(spec.Fields) != 21 {
			b.Fatalf("unexpected import: %d, %v", len(spec.Fields), err)
		}
		capacity = cap(spec.Fields)
	}
	b.ReportMetric(float64(capacity), "field-cap")
}

func BenchmarkImportDefinitionScalarFields(b *testing.B) {
	var fields strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&fields, "new StringField('field_%d', 'field%d'),\n", i, i)
	}
	source := associationImportSource(fields.String())
	b.ReportAllocs()
	var capacity int
	for b.Loop() {
		spec, err := ImportDefinition(source, nil)
		if err != nil || len(spec.Fields) != 41 {
			b.Fatalf("unexpected import: %d, %v", len(spec.Fields), err)
		}
		capacity = cap(spec.Fields)
	}
	b.ReportMetric(float64(capacity), "field-cap")
}

func TestImportDefinitionRejectsPartialDynamicCollections(t *testing.T) {
	for _, statement := range []string{
		"$fields[] = customFieldFactory();",
		"$fields[0] = customFieldFactory();",
		"$fields = dynamicFields();",
	} {
		t.Run(statement, func(t *testing.T) {
			source := associationImportSource("")
			source = strings.Replace(source, "return new FieldCollection([new IdField('id', 'id'), ]);", "$fields = [new IdField('id', 'id')];\n"+statement+"\nreturn new FieldCollection($fields);", 1)
			spec, err := ImportDefinition(source, nil)
			require.ErrorContains(t, err, "FieldCollection has no literal array")
			require.Empty(t, spec.Fields, "a partial field list must not be exposed as editable")
		})
	}
}
