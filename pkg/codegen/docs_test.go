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

package codegen

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const codeFence = "```"

func TestFilterExamples(t *testing.T) {
	t.Parallel()

	tsCodeSnippet := `### Example 1
` + codeFence + `typescript
import * as path from path;

console.log("I am a console log statement in ts.");
` + codeFence

	goCodeSnippet := `\n` + codeFence + `go
import (
	"fmt"
	"strings"
)

func fakeFunc() {
	fmt.Print("Hi, I am a fake func!")
}
` + codeFence

	leadingDescription := "This is a leading description for this resource."
	exampleShortCode := `{{% example %}}` + tsCodeSnippet + "\n" + goCodeSnippet + `{{% /example %}}`
	description := leadingDescription + `
{{% examples %}}` + exampleShortCode + `
{{% /examples %}}`

	t.Run("ContainsRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "typescript")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, leadingDescription, "expected to at least find the leading description")
	})

	// The above description does not contain a Python code snippet and because
	// the description contains only one Example without any Python code snippet,
	// we should expect an empty string in this test.
	t.Run("DoesNotContainRelevantSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "python")
		assert.Contains(t, strippedDescription, leadingDescription, "expected to at least find the leading description")
		// Should not contain any examples sections.
		assert.NotContains(t, strippedDescription, "### ", "expected to not have any examples but found at least one")
	})
}

func TestTestFilterExamplesFromMultipleExampleSections(t *testing.T) {
	t.Parallel()

	tsCodeSnippet := codeFence + `typescript
import * as path from path;

console.log("I am a console log statement in ts.");
` + codeFence

	goCodeSnippet := codeFence + `go
import (
	"fmt"
	"strings"
)

func fakeFunc() {
	fmt.Print("Hi, I am a fake func!")
}
` + codeFence

	example1 := `### Example 1
` + tsCodeSnippet + "\n" + goCodeSnippet

	example2 := `### Example 2
` + tsCodeSnippet

	example1ShortCode := `{{% example %}}` + "\n" + example1 + "\n" + `{{% /example %}}`
	example2ShortCode := `{{% example %}}` + "\n" + example2 + "\n" + `{{% /example %}}`
	description := `Some other {{% ref #/resources/aws:s3:bucket %}} shortcode content.` +
		` {{% examples %}}` + "\n" + example1ShortCode + "\n" + example2ShortCode + "\n" + `{{% /examples %}}`

	t.Run("EveryExampleHasRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "typescript")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Some other {{% ref #/resources/aws:s3:bucket %}} shortcode content.")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.Contains(t, strippedDescription, "Example 2", "expected Example 2 section")
	})

	t.Run("SomeExamplesHaveRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "go")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Some other {{% ref #/resources/aws:s3:bucket %}} shortcode content.")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.NotContains(t, strippedDescription, "Example 2",
			"unexpected Example 2 section. section should have been excluded")
	})
}

// bindTestPackage binds a small package spec suitable for testing InterpretPulumiRefs.
// It includes a resource test:s3:Bucket (with output property "region") and a resource
// test:mod:Resource (with output property "myProperty").
func bindTestPackage(t *testing.T) *schema.Package {
	t.Helper()
	spec := schema.PackageSpec{
		Name:    "test",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"test:s3:Bucket": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"region": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
			},
			"test:mod:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"myProperty": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
				},
			},
		},
	}
	pkg, err := schema.ImportSpec(spec, nil, schema.ValidationOptions{})
	require.NoError(t, err)
	return pkg
}

func TestInterpretPulumiRefs(t *testing.T) {
	t.Parallel()

	t.Run("ResolvesKnownRefs", func(t *testing.T) {
		t.Parallel()

		pkg := bindTestPackage(t)
		description := "This is a reference to {{% ref #/resources/test:s3:Bucket %}} and one to the " +
			"{{% ref #/resources/test:s3:Bucket/properties/region %}} property."
		expected := "This is a reference to s3.Bucket and one to the region property.\n"
		result, err := pkg.InterpretPulumiRefs(description, func(ref schema.DocRef) (string, bool) {
			if ref.Kind == schema.DocRefKindResource {
				rt := ref.Type.(*schema.ResourceType)
				tok := tokens.Type(rt.Token)
				return tok.Module().Name().String() + "." + tok.Name().String(), true
			}
			return "", false
		})
		require.NoError(t, err)
		assert.Equal(t, expected, result, "expected resolved references")
	})

	t.Run("FallsBackWhenNotMapped", func(t *testing.T) {
		t.Parallel()

		pkg := bindTestPackage(t)
		description := "This is a reference to {{% ref #/resources/test:mod:Resource/properties/myProperty %}}" +
			" and to {{% ref #/resources/test:mod:Resource %}}."
		expected := "This is a reference to myProperty and to test:mod:Resource.\n"
		result, err := pkg.InterpretPulumiRefs(description, func(ref schema.DocRef) (string, bool) {
			return "", false
		})
		require.NoError(t, err)
		assert.Equal(t, expected, result, "expected fallback to last segment for unknown references")
	})

	t.Run("HandlesEmptyDescription", func(t *testing.T) {
		t.Parallel()

		pkg := bindTestPackage(t)
		result, err := pkg.InterpretPulumiRefs("", func(ref schema.DocRef) (string, bool) {
			return "ResolvedName", true
		})
		require.NoError(t, err)
		assert.Equal(t, "", result, "expected empty result for empty description")
	})

	t.Run("HandlesNoRefsInDescription", func(t *testing.T) {
		t.Parallel()

		pkg := bindTestPackage(t)
		description := "This description has no Pulumi references."
		expected := "This description has no Pulumi references.\n"
		result, err := pkg.InterpretPulumiRefs(description, func(ref schema.DocRef) (string, bool) {
			return "ResolvedName", true
		})
		require.NoError(t, err)
		assert.Equal(t, expected, result, "expected unchanged description when no refs are present")
	})

	t.Run("ErrorsIfRefIsMalformed", func(t *testing.T) {
		t.Parallel()

		pkg := bindTestPackage(t)
		description := "This is a {{% ref bad %}} reference."
		_, err := pkg.InterpretPulumiRefs(description, func(ref schema.DocRef) (string, bool) {
			return "ResolvedName", true
		})
		require.ErrorContains(t, err, "invalid doc ref: bad")
	})
}
