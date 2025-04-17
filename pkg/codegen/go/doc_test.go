// Copyright 2016-2020, Pulumi Corporation.
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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package gen

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPackageSpec = schema.PackageSpec{
	Name:        "aws",
	Version:     "0.0.1",
	Description: "A fake provider package used for testing.",
	Meta: &schema.MetadataSpec{
		ModuleFormat: "(.*)(?:/[^/]*)",
	},
	Types: map[string]schema.ComplexTypeSpec{
		"aws:s3/BucketCorsRule:BucketCorsRule": {
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Description: "The resource options object.",
				Type:        "object",
				Properties: map[string]schema.PropertySpec{
					"stringProp": {
						Description: "A string prop.",
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
	},
	Resources: map[string]schema.ResourceSpec{
		"aws:s3/bucket:Bucket": {
			InputProperties: map[string]schema.PropertySpec{
				"corsRules": {
					TypeSpec: schema.TypeSpec{
						Ref: "#/types/aws:s3/BucketCorsRule:BucketCorsRule",
					},
				},
			},
		},
	},
}

func getTestPackage(t *testing.T) *schema.Package {
	t.Helper()

	pkg, err := schema.ImportSpec(testPackageSpec, nil, false)
	assert.NoError(t, err, "could not import the test package spec")
	return pkg
}

func TestGetDocLinkForPulumiType(t *testing.T) {
	t.Parallel()

	t.Run("Generate_ResourceOptionsLink_Specified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		pkg.Language["go"] = GoPackageInfo{PulumiSDKVersion: 1}
		expected := "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/go/pulumi?tab=doc#ResourceOption"
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
	t.Run("Generate_ResourceOptionsLink_Specified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		pkg.Language["go"] = GoPackageInfo{PulumiSDKVersion: 2}
		expected := "https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v2/go/pulumi?tab=doc#ResourceOption"
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
	t.Run("Generate_ResourceOptionsLink_Unspecified", func(t *testing.T) {
		t.Parallel()

		pkg := getTestPackage(t)
		d := DocLanguageHelper{}
		expected := fmt.Sprintf("https://pkg.go.dev/github.com/pulumi/pulumi/sdk/%s/go/pulumi?tab=doc#ResourceOption", pulumiSDKVersion)
		link := d.GetDocLinkForPulumiType(pkg, "ResourceOption")
		assert.Equal(t, expected, link)
	})
}

func TestGetDocLinkForResourceType(t *testing.T) {
	t.Parallel()

	pkg := getTestPackage(t)
	d := DocLanguageHelper{}
	expected := "https://pkg.go.dev/github.com/pulumi/pulumi-aws/sdk/go/aws/s3?tab=doc#Bucket"
	link := d.GetDocLinkForResourceType(pkg, "s3", "Bucket")
	assert.Equal(t, expected, link)
}

func TestGetFunctionName(t *testing.T) {
	t.Parallel()
	pkg, err := schema.ImportSpec(schema.PackageSpec{
		Name:    "pkg",
		Version: "0.0.1",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Resources: map[string]schema.ResourceSpec{
			"pkg:conflict:Resource": {},
		},
		Functions: map[string]schema.FunctionSpec{
			"pkg:index:getSomeFunction": {},
			"pkg:conflict:newResource":  {},
		},
	}, nil)
	require.NoError(t, err)
	d := DocLanguageHelper{}
	d.GeneratePackagesMap(pkg, "test", GoPackageInfo{})

	names := map[string]string{}
	for _, f := range pkg.Functions {
		names[f.Token] = d.GetFunctionName(f)
	}

	assert.Equal(t, map[string]string{
		"pkg:index:getSomeFunction": "GetSomeFunction",
		// "pkg:conflict:newResource" is renamed to "CreateResource" to avoid
		// conflicting with the resource constructor for "pkg:conflict:Resource"
		// (NewResource).
		"pkg:conflict:newResource": "CreateResource",
	}, names)
}

// Calling GetFunctionName may return the wrong result when
// [DocLanguageHelper.GeneratePackagesMap] is not called, but it shouldn't panic.
func TestGetFunctionNameWithoutPackageMapDoesNotPanic(t *testing.T) {
	t.Parallel()

	pkg, err := schema.ImportSpec(schema.PackageSpec{
		Name:    "pkg",
		Version: "0.0.1",
		Meta: &schema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
		Functions: map[string]schema.FunctionSpec{
			"pkg:index:getSomeFunction": {},
		},
	}, nil)
	require.NoError(t, err)
	d := DocLanguageHelper{}

	assert.Equal(t, "GetSomeFunction", d.GetFunctionName(pkg.Functions[0]))
}
