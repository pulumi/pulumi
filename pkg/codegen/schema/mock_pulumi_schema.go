package schema

import (
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
						"outputs": {TypeSpec: TypeSpec{Type: "object"}},
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
					TypeSpec: TypeSpec{
						Type: "string",
					},
				},
			},
		},
		// Provider: ResourceSpec{
		// 	ObjectTypeSpec: ObjectTypeSpec{
		// 	InputProperties: map[string]PropertySpec{
		// 		"Name": {
		// 			TypeSpec: TypeSpec{
		// 				Type:      "string",
		// 			},
		// 		},
		// 	},

	}

	pkg, err := ImportSpec(spec, nil)
	contract.AssertNoError(err)
	return pkg
}

var DefaultPulumiPackage = newPulumiPackage()
