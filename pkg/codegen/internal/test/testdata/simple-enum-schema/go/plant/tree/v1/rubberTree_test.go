package v1

import (
	"github.com/pulumi/pulumi/pkg/v2/codegen/internal/test/testdata/simple-enum-schema/go/plant"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type mocks int

func (mocks) NewResource(typeToken, name string, inputs resource.PropertyMap, provider, id string) (string, resource.PropertyMap, error) {
	return name + "_id", inputs, nil
}

func (mocks) Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	return args, nil
}

func TestRubberTree(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := NewRubberTree(ctx, "blah", &RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    plant.Red,
					Material: pulumi.String("ceramic"),
					Size:     plant.FourInch,
				},
				Farm: Plants_R_Us,
				Type: Ruby,
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(tree.URN(), tree.Container.Material(), tree.Container.Color(), tree.Container.Size(), tree.Type).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*int)
				typ := all[4].(string)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "red", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, 4, "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, "Ruby", "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0)))
	})

	t.Run("StringsForRelaxedEnum", func(t *testing.T) {
		pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := NewRubberTree(ctx, "blah", &RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: Plants_R_Us,
				Type: Ruby,
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(tree.URN(), tree.Container.Material(), tree.Container.Color(), tree.Container.Size(), tree.Type).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*int)
				typ := all[4].(string)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "Magenta", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, 22, "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, "Ruby", "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1)))
	})

	t.Run("StringsForStrictEnum", func(t *testing.T) {
		pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := NewRubberTree(ctx, "blah", &RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: Plants_R_Us,
				Type: "Burgundy",
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(tree.URN(), tree.Container.Material(), tree.Container.Color(), tree.Container.Size(), tree.Type).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*int)
				typ := all[4].(string)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "Magenta", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, 22, "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, "Burgundy", "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1)))
	})

	t.Run("ValidateStrictEnum", func(t *testing.T) {
		pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := NewRubberTree(ctx, "blah", &RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    plant.Red,
					Material: pulumi.String("ceramic"),
				},
				Farm: Plants_R_Us,
				Type: "Mauve",
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid value for enum 'Type'")
			require.Nil(t, tree)
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(2)))
	})
}
