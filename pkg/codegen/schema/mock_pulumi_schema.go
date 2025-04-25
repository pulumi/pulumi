// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	pkg, diags, err := bindSpec(spec, nil, nullLoader{}, false, ValidationOptions{})
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
