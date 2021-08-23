package gen

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/test/testdata/simple-enum-schema/go/plant"
	tree "github.com/pulumi/pulumi/pkg/v3/codegen/internal/test/testdata/simple-enum-schema/go/plant/tree/v1"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	generatePackage := func(tool string, pkg *schema.Package, files map[string][]byte) (map[string][]byte, error) {
		return GeneratePackage(tool, pkg)
	}
	test.TestSDKCodegen(t, "go", generatePackage)
}

func TestGenerateTypeNames(t *testing.T) {
	test.TestTypeNameCodegen(t, "go", func(pkg *schema.Package) test.TypeNameGeneratorFunc {
		err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer})
		require.NoError(t, err)

		var goPkgInfo GoPackageInfo
		if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
			goPkgInfo = goInfo
		}
		packages := generatePackageContextMap("test", pkg, goPkgInfo)

		root, ok := packages[""]
		require.True(t, ok)

		return func(t schema.Type) string {
			return root.typeString(t)
		}
	})
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
				assert.Nil(t, brightness)
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

func TestGenerateOutputFuncs(t *testing.T) {
	testDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs")

	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	var examples []string
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".json") {
			examples = append(examples, strings.TrimSuffix(name, ".json"))
		}
	}

	sort.Slice(examples, func(i, j int) bool { return examples[i] < examples[j] })

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
			expectedOutputFile := filepath.Join(testDir, "go", fmt.Sprintf("%s.go", ex))
			test.ValidateFileTransformer(t, inputFile, expectedOutputFile, gen)
		})
	}

	goDir := filepath.Join("..", "internal", "test", "testdata", "output-funcs", "go")

	t.Run("compileGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && go mod tidy", goDir)
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())

		t.Logf("cd %s && go build .", goDir)
		cmd = exec.Command("go", "build", ".")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())
	})

	t.Run("testGeneratedCode", func(t *testing.T) {
		t.Logf("cd %s && go test .", goDir)
		cmd := exec.Command("go", "test", ".")
		cmd.Dir = goDir
		assert.NoError(t, cmd.Run())
	})
}
