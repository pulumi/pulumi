// Copyright 2026, Pulumi Corporation.
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

package pcl_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// multiExtensionLoader serves a bare base provider ("extbase") plus two
// extensions layered on it ("exta", "extb"), each defining one distinct
// resource in the base provider's namespace. All three resource tokens share
// the "extbase:" prefix; only the schema says who owns which.
type multiExtensionLoader struct{}

func (l multiExtensionLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	return l.LoadPackageV2(context.TODO(), &schema.PackageDescriptor{Name: pkg, Version: version})
}

func (multiExtensionLoader) LoadPackageV2(
	_ context.Context, d *schema.PackageDescriptor,
) (*schema.Package, error) {
	var spec schema.PackageSpec
	switch {
	case d.Parameterization == nil:
		// The bare base provider owns extbase:index:Base.
		spec = schema.PackageSpec{
			Name:    "extbase",
			Version: "45.0.0",
			Resources: map[string]schema.ResourceSpec{
				"extbase:index:Base": {ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}},
			},
		}
	case d.Parameterization.Name == "exta":
		spec = extensionSpecFor("exta", "extbase:index:Aye")
	case d.Parameterization.Name == "extb":
		spec = extensionSpecFor("extb", "extbase:index:Bee")
	default:
		return nil, fmt.Errorf("unexpected package %q", d.PackageName())
	}

	pkg, diags, err := schema.BindSpec(spec, schema.NewNullLoader(), schema.ValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return pkg, nil
}

// extensionSpecFor builds an extension package named `name` that defines one
// resource `token` in the base provider's ("extbase") namespace.
func extensionSpecFor(name, token string) schema.PackageSpec {
	return schema.PackageSpec{
		Name:    name,
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			token: {ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}},
		},
		ExtensionParameterization: &schema.ExtensionParameterizationSpec{
			BaseProvider: schema.BaseProviderRefSpec{Name: "extbase", Version: "45.0.0"},
			Parameter:    []byte("x"),
		},
	}
}

func TestBindMultiExtensionAndBaseOverSameBase(t *testing.T) {
	t.Parallel()

	source := `
package {
    baseProviderName = "extbase"
    baseProviderVersion = "45.0.0"
    parameterization {
        name = "exta"
        version = "1.0.0"
        value = "eA=="
    }
}

package {
    baseProviderName = "extbase"
    baseProviderVersion = "45.0.0"
    parameterization {
        name = "extb"
        version = "1.0.0"
        value = "eA=="
    }
}

resource a "extbase:index:Aye" { }

resource b "extbase:index:Bee" { }

resource base "extbase:index:Base" { }
`

	parser := syntax.NewParser()
	require.NoError(t, parser.ParseFile(bytes.NewReader([]byte(source)), "program.pp"))
	require.False(t, parser.Diagnostics.HasErrors(), "parse: %v", parser.Diagnostics)

	program, diags, err := pcl.BindProgram(parser.Files, multiExtensionLoader{})
	require.NoError(t, err)
	require.NotNil(t, program, "program should bind")
	require.False(t, diags.HasErrors(),
		"all three resources should resolve (Aye->exta, Bee->extb, Base->bare extbase); diags: %v", diags)
}
