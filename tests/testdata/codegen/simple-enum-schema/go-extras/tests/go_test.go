package tests

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"simple-enum-schema/plant"
	tree "simple-enum-schema/plant/tree/v1"
)

func TestEnumUsage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			rubberTree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: &plant.ContainerArgs{
					Color:    plant.ContainerColorRed,
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSizeFourInch,
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: tree.RubberTreeVarietyRuby,
			})
			require.NoError(t, err)
			require.NotNil(t, rubberTree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				rubberTree.URN(),
				rubberTree.Container.Material(),
				rubberTree.Container.Color(),
				rubberTree.Container.Size(),
				rubberTree.Container.Brightness(),
				rubberTree.Type,
			).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*plant.ContainerSize)
				brightness := all[4].(*plant.ContainerBrightness)
				typ := all[5].(tree.RubberTreeVariety)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "red", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, plant.ContainerSizeFourInch, "unexpected size on resource: %v", urn)
				assert.Equal(t, *brightness, plant.ContainerBrightness(1.0))
				assert.Equal(t, typ, tree.RubberTreeVarietyRuby, "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0))))
	})

	t.Run("StringsForRelaxedEnum", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			rubberTree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: tree.RubberTreeVarietyRuby,
			})
			require.NoError(t, err)
			require.NotNil(t, rubberTree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				rubberTree.URN(),
				rubberTree.Container.Material(),
				rubberTree.Container.Color(),
				rubberTree.Container.Size(),
				rubberTree.Type,
			).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*plant.ContainerSize)
				typ := all[4].(tree.RubberTreeVariety)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "Magenta", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, plant.ContainerSize(22), "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, tree.RubberTreeVarietyRuby, "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})

	t.Run("StringsForStrictEnum", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			rubberTree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: tree.RubberTreeVarietyBurgundy,
			})
			require.NoError(t, err)
			require.NotNil(t, rubberTree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				rubberTree.URN(),
				rubberTree.Container.Material(),
				rubberTree.Container.Color(),
				rubberTree.Container.Size(),
				rubberTree.Type,
			).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*plant.ContainerSize)
				typ := all[4].(tree.RubberTreeVariety)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "Magenta", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, plant.ContainerSize(22), "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, tree.RubberTreeVarietyBurgundy, "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})

	t.Run("EnumOutputs", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			rubberTree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    plant.ContainerColor("Magenta").ToContainerColorOutput().ToStringOutput(),
					Material: pulumi.String("ceramic").ToStringOutput(),
					Size:     plant.ContainerSize(22).ToContainerSizeOutput(),
				},
				Farm: tree.Farm_Plants_R_Us.ToFarmPtrOutput().ToStringPtrOutput(),
				Type: tree.RubberTreeVarietyBurgundy.ToRubberTreeVarietyOutput(),
			})
			require.NoError(t, err)
			require.NotNil(t, rubberTree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				rubberTree.URN(),
				rubberTree.Container.Material(),
				rubberTree.Container.Color(),
				rubberTree.Container.Size(),
				rubberTree.Type,
			).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*plant.ContainerSize)
				typ := all[4].(tree.RubberTreeVariety)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "Magenta", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, plant.ContainerSize(22), "unexpected size on resource: %v", urn)
				assert.Equal(t, typ, tree.RubberTreeVarietyBurgundy, "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})
}

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
