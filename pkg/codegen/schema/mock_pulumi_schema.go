package schema

import (
	"context"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newPulumiPackage() *Package {
	spec := PackageSpec{
		Name:        "pulumi",
		DisplayName: "Pulumi",
		Version:     "1.0.0",
		Description: "mock pulumi package",
		Resources: map[string]ResourceSpec{
			"pulumi:pulumi:StackReference": {
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"outputs": {TypeSpec: TypeSpec{
							Type: "object",
							AdditionalProperties: &TypeSpec{
								Ref: "pulumi.json#/Any",
							},
						}},
					},
					Required: []string{
						"outputs",
					},
				},
				InputProperties: map[string]PropertySpec{
					"name": {TypeSpec: TypeSpec{Type: "string"}},
				},
			},
		},
		Provider: ResourceSpec{
			InputProperties: map[string]PropertySpec{
				"name": {
					Description: "fully qualified name of stack, i.e. <organization>/<project>/<stack>",
					TypeSpec: TypeSpec{
						Type: "string",
					},
				},
			},
		},
	}

	pkg, diags, err := bindSpec(spec, nil, nullLoader{}, false)
	if err == nil && diags.HasErrors() {
		err = diags
	}
	contract.AssertNoErrorf(err, "failed to bind mock pulumi package")
	return pkg
}

type nullLoader struct{}

func (nullLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	contract.Failf("nullLoader invoked on %s,%s", pkg, version)
	return nil, nil
}

func (nullLoader) LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error) {
	contract.Failf("nullLoader invoked on %s,%s", descriptor.Name, descriptor.Version)
	return nil, nil
}

var DefaultPulumiPackage = newPulumiPackage()
