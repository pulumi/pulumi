package gen

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test/testdata/simple-enum-schema/go/plant"
	tree "github.com/pulumi/pulumi/pkg/v3/codegen/internal/test/testdata/simple-enum-schema/go/plant/tree/v1"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputUsage(t *testing.T) {
	pkg := &pkgContext{}
	arrayUsage := pkg.getInputUsage("FooArray")
	assert.Equal(
		t,
		"FooArrayInput is an input type that accepts FooArray and FooArrayOutput values.\nYou can construct a "+
			"concrete instance of `FooArrayInput` via:\n\n\t\t FooArray{ FooArgs{...} }\n ",
		arrayUsage)

	mapUsage := pkg.getInputUsage("FooMap")
	assert.Equal(
		t,
		"FooMapInput is an input type that accepts FooMap and FooMapOutput values.\nYou can construct a concrete"+
			" instance of `FooMapInput` via:\n\n\t\t FooMap{ \"key\": FooArgs{...} }\n ",
		mapUsage)

	ptrUsage := pkg.getInputUsage("FooPtr")
	assert.Equal(
		t,
		"FooPtrInput is an input type that accepts FooArgs, FooPtr and FooPtrOutput values.\nYou can construct a "+
			"concrete instance of `FooPtrInput` via:\n\n\t\t FooArgs{...}\n\n or:\n\n\t\t nil\n ",
		ptrUsage)

	usage := pkg.getInputUsage("Foo")
	assert.Equal(
		t,
		"FooInput is an input type that accepts FooArgs and FooOutput values.\nYou can construct a concrete instance"+
			" of `FooInput` via:\n\n\t\t FooArgs{...}\n ",
		usage)
}

func TestGoPackageName(t *testing.T) {
	assert.Equal(t, "aws", goPackage("aws"))
	assert.Equal(t, "azure", goPackage("azure-nextgen"))
	assert.Equal(t, "plant", goPackage("plant-provider"))
	assert.Equal(t, "", goPackage(""))
}

func TestGeneratePackage(t *testing.T) {
	tests := []struct {
		name                      string
		schemaDir                 string
		expectedFiles             []string
		genResourceContainerTypes bool
	}{
		{
			"Simple schema with local resource properties",
			"simple-resource-schema",
			[]string{
				filepath.Join("example", "argFunction.go"),
				filepath.Join("example", "doc.go"),
				filepath.Join("example", "init.go"),
				filepath.Join("example", "otherResource.go"),
				filepath.Join("example", "provider.go"),
				filepath.Join("example", "pulumiTypes.go"),
				filepath.Join("example", "pulumiUtilities.go"),
				filepath.Join("example", "resource.go"),
			},
			false,
		},
		{
			"Simple schema with enum types",
			"simple-enum-schema",
			[]string{
				filepath.Join("plant", "doc.go"),
				filepath.Join("plant", "init.go"),
				filepath.Join("plant", "provider.go"),
				filepath.Join("plant", "pulumiTypes.go"),
				filepath.Join("plant", "pulumiUtilities.go"),
				filepath.Join("plant", "pulumiEnums.go"),
				filepath.Join("plant", "provider.go"),
				filepath.Join("plant", "tree", "v1", "init.go"),
				filepath.Join("plant", "tree", "v1", "rubberTree.go"),
				filepath.Join("plant", "tree", "v1", "pulumiEnums.go"),
				filepath.Join("plant", "tree", "v1", "nursery.go"),
			},
			false,
		},
		{
			"External resource schema",
			"external-resource-schema",
			[]string{
				filepath.Join("example", "init.go"),
				filepath.Join("example", "argFunction.go"),
				filepath.Join("example", "cat.go"),
				filepath.Join("example", "component.go"),
				filepath.Join("example", "doc.go"),
				filepath.Join("example", "provider.go"),
				filepath.Join("example", "pulumiTypes.go"),
				filepath.Join("example", "pulumiUtilities.go"),
				filepath.Join("example", "workload.go"),
			},
			true,
		},
		{
			"Simple schema with plain properties",
			"simple-plain-schema",
			[]string{
				filepath.Join("example", "doc.go"),
				filepath.Join("example", "init.go"),
				filepath.Join("example", "component.go"),
				filepath.Join("example", "provider.go"),
				filepath.Join("example", "pulumiTypes.go"),
				filepath.Join("example", "pulumiUtilities.go"),
			},
			false,
		},
		{
			"Simple schema with root package set",
			"simple-plain-schema-with-root-package",
			[]string{
				filepath.Join("doc.go"),
				filepath.Join("init.go"),
				filepath.Join("component.go"),
				filepath.Join("provider.go"),
				filepath.Join("pulumiTypes.go"),
				filepath.Join("pulumiUtilities.go"),
			},
			false,
		},
	}
	testDir := filepath.Join("..", "internal", "test", "testdata")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := test.GeneratePackageFilesFromSchema(
				filepath.Join(testDir, tt.schemaDir, "schema.json"),
				func(tool string, pkg *schema.Package, files map[string][]byte) (map[string][]byte, error) {
					return GeneratePackage(tool, pkg)
				})
			assert.NoError(t, err)
			dir := filepath.Join(testDir, tt.schemaDir)
			test.RewriteFilesWhenPulumiAccept(t, dir, "go", files)
			expectedFiles, err := test.LoadFiles(dir, "go", tt.expectedFiles)
			assert.NoError(t, err)
			test.ValidateFileEquality(t, files, expectedFiles)
			test.CheckAllFilesGenerated(t, files, expectedFiles)
		})
	}
}

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestEnumUsage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: &plant.ContainerArgs{
					Color:    plant.ContainerColorRed,
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSizeFourInch,
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: tree.RubberTreeVarietyRuby,
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				tree.URN(),
				tree.Container.Material(),
				tree.Container.Color(),
				tree.Container.Size(),
				tree.Container.Brightness(),
				tree.Type,
			).ApplyT(func(all []interface{}) error {
				urn := all[0].(pulumi.URN)
				material := all[1].(*string)
				color := all[2].(*string)
				size := all[3].(*int)
				brightness := all[4].(*float64)
				typ := all[5].(string)
				assert.Equal(t, *material, "ceramic", "unexpected material on resource: %v", urn)
				assert.Equal(t, *color, "red", "unexpected color on resource: %v", urn)
				assert.Equal(t, *size, 4, "unexpected size on resource: %v", urn)
				assert.Nil(t, brightness)
				assert.Equal(t, typ, "Ruby", "unexpected type on resource: %v", urn)
				wg.Done()
				return nil
			})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(0))))
	})

	t.Run("StringsForRelaxedEnum", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: tree.RubberTreeVarietyRuby,
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				tree.URN(), tree.Container.Material(), tree.Container.Color(), tree.Container.Size(), tree.Type,
			).ApplyT(func(all []interface{}) error {
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
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})

	t.Run("StringsForStrictEnum", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			tree, err := tree.NewRubberTree(ctx, "blah", &tree.RubberTreeArgs{
				Container: plant.ContainerArgs{
					Color:    pulumi.String("Magenta"),
					Material: pulumi.String("ceramic"),
					Size:     plant.ContainerSize(22),
				},
				Farm: tree.Farm_Plants_R_Us,
				Type: "Burgundy",
			})
			require.NoError(t, err)
			require.NotNil(t, tree)
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(
				tree.URN(), tree.Container.Material(), tree.Container.Color(), tree.Container.Size(), tree.Type,
			).ApplyT(func(all []interface{}) error {
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
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})
}

func TestGenerateOutputFuncs(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	examples := []string{
		"listStorageAccountKeys",
		"getClientConfig",
		"getIntegrationRuntimeObjectMetadatum",
		"funcWithConstInput",
	}

	gen := func(reader io.Reader, writer io.Writer) error {
		var pkgSpec schema.PackageSpec
		err := json.NewDecoder(reader).Decode(&pkgSpec)
		if err != nil {
			return err
		}
		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			return err
		}

		tool := "tool"
		var goPkgInfo GoPackageInfo
		if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
			goPkgInfo = goInfo
		}
		pkgContexts := generatePackageContextMap(tool, pkg, goPkgInfo)

		var pkgContext *pkgContext

		for _, c := range pkgContexts {
			if len(c.functionNames) == 1 {
				pkgContext = c
			}
		}

		if pkgContext == nil {
			return fmt.Errorf("Cannot find a package with 1 function in generatePackageContextMap result")
		}

		fun := pkg.Functions[0]
		_, err = writer.Write([]byte(pkgContext.genFunctionCodeFile(fun)))
		return err
	}

	for _, ex := range examples {
		t.Run(ex, func(t *testing.T) {
			inputFile := filepath.Join(testDir, fmt.Sprintf("%s.json", ex))
			expectedOutputFile := filepath.Join(testDir, fmt.Sprintf("%s.go", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}
}
