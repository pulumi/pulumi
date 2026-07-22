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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// redirectLoader serves a package that declares members under a foreign package
// name, the way allowedPackageNames permits. "other:mod:Res" has no package of
// its own to resolve against; only a provider option can reach it.
type redirectLoader struct{}

func (l redirectLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	return l.LoadPackageV2(context.TODO(), &schema.PackageDescriptor{Name: pkg, Version: version})
}

func (redirectLoader) LoadPackageV2(_ context.Context, d *schema.PackageDescriptor) (*schema.Package, error) {
	if d.Name != "redirected" {
		return nil, unknownPackageError{d.Name}
	}
	spec := schema.PackageSpec{
		Name:                "redirected",
		Version:             "1.0.0",
		AllowedPackageNames: []string{"other"},
		Resources: map[string]schema.ResourceSpec{
			"other:mod:Res": {ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}},
		},
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

type unknownPackageError struct{ name string }

func (e unknownPackageError) Error() string { return "unknown package " + e.name }

func bindRedirectProgram(t *testing.T, source string) (*pcl.Program, error) {
	t.Helper()

	parser := syntax.NewParser()
	require.NoError(t, parser.ParseFile(bytes.NewReader([]byte(source)), "program.pp"))
	require.False(t, parser.Diagnostics.HasErrors(), "parse: %v", parser.Diagnostics)

	program, diags, err := pcl.BindProgram(parser.Files, redirectLoader{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return program, nil
}

// A token naming a foreign package binds against the package named by the
// provider option, and the program depends on that package rather than on the
// phantom package the token names.
func TestBindForeignTokenThroughProviderOption(t *testing.T) {
	t.Parallel()

	program, err := bindRedirectProgram(t, `
package {
    baseProviderName = "redirected"
    baseProviderVersion = "1.0.0"
}

resource "res" "other:mod:Res" {
    options {
        provider = redirected
    }
}
`)
	require.NoError(t, err)

	packages, err := program.PackageSnapshots()
	require.NoError(t, err)
	names := make([]string, 0, len(packages))
	for _, p := range packages {
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"redirected"}, names)
}

// The provider option decides where the token is looked up, so a token the
// named package does not define is an error. It must not fall back to
// resolving the token by its own package portion.
func TestBindUnknownTokenThroughProviderOptionFails(t *testing.T) {
	t.Parallel()

	_, err := bindRedirectProgram(t, `
package {
    baseProviderName = "redirected"
    baseProviderVersion = "1.0.0"
}

resource "res" "other:mod:Missing" {
    options {
        provider = redirected
    }
}
`)
	require.ErrorContains(t, err, "unknown resource type 'other:mod:Missing'")
}
