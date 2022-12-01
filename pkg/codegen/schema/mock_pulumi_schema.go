package schema

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newPulumiPackage() *Package {
	md := func(s string) DescriptionSpec {
		spec, err := MakeMarkdownDescription(s).marshal()
		contract.AssertNoError(err)
		return spec
	}
	spec := PackageSpec{
		Name:        "pulumi",
		DisplayName: "Pulumi",
		Version:     "1.0.0",
		Description: md("mock pulumi package"),
		Resources: map[string]ResourceSpec{
			"pulumi:pulumi:StackReference": {
				ObjectTypeSpec: ObjectTypeSpec{
					Properties: map[string]PropertySpec{
						"outputs": {TypeSpec: TypeSpec{Type: "object"}},
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
					Description: md("fully qualified name of stack, i.e. <organization>/<project>/<stack>"),
					TypeSpec: TypeSpec{
						Type: "string",
					},
				},
			},
		},
	}

	pkg, err := ImportSpec(spec, nil)
	contract.AssertNoError(err)
	return pkg
}

var DefaultPulumiPackage = newPulumiPackage()
