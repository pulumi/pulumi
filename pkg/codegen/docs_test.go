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

	"github.com/stretchr/testify/assert"
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
	description := `{{% examples %}}` + "\n" + example1ShortCode + "\n" + example2ShortCode + "\n" + `{{% /examples %}}`

	t.Run("EveryExampleHasRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "typescript")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.Contains(t, strippedDescription, "Example 2", "expected Example 2 section")
	})

	t.Run("SomeExamplesHaveRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		strippedDescription := FilterExamples(description, "go")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.NotContains(t, strippedDescription, "Example 2",
			"unexpected Example 2 section. section should have been excluded")
	})
}

func TestInterpretPulumiRefs(t *testing.T) {
	t.Parallel()

	t.Run("ResolvesKnownRefs", func(t *testing.T) {
		t.Parallel()

		description := "This is a reference to <pulumi ref=\"#/resources/aws:s3:bucket\" /> and one to the " +
			"<pulumi ref=\"#/resources/azure:storage:storageAccount/properties/region\" /> property."
		expected := "This is a reference to s3.bucket and one to the region property."
		result := InterpretPulumiRefs(description, func(ref DocRef) (string, bool) {
			//nolint:exhaustive
			switch ref.Type {
			case DocRefTypeResource:
				return ref.Token.Module().Name().String() + "." + ref.Token.Name().String(), true
			default:
				return "", false
			}
		})
		assert.Equal(t, expected, result, "expected resolved references")
	})

	t.Run("FallsBackWhenNotMapped", func(t *testing.T) {
		t.Parallel()

		description := "This is a reference to <pulumi ref=\"#/resources/unknown:resource:type/properties/myProperty\"/>" +
			" and to <pulumi ref=\"#/resources/unknown:resource:type\" />."
		expected := "This is a reference to myProperty and to unknown:resource:type."
		result := InterpretPulumiRefs(description, func(ref DocRef) (string, bool) {
			return "", false
		})
		assert.Equal(t, expected, result, "expected fallback to last segment for unknown references")
	})

	t.Run("HandlesEmptyDescription", func(t *testing.T) {
		t.Parallel()

		description := ""
		expected := ""
		result := InterpretPulumiRefs(description, func(ref DocRef) (string, bool) {
			return "ResolvedName", true
		})
		assert.Equal(t, expected, result, "expected empty result for empty description")
	})

	t.Run("HandlesNoRefsInDescription", func(t *testing.T) {
		t.Parallel()

		description := "This description has no Pulumi references."
		expected := "This description has no Pulumi references."
		result := InterpretPulumiRefs(description, func(ref DocRef) (string, bool) {
			return "ResolvedName", true
		})
		assert.Equal(t, expected, result, "expected unchanged description when no refs are present")
	})
}

func TestParseDocRef(t *testing.T) {
	t.Parallel()

	t.Run("InvalidUnknownTopLevelType", func(t *testing.T) {
		t.Parallel()
		ref := "#/unknown/aws:s3:bucket"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidTopLevelOnly", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidMissingToken", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("ResourceRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket"
		expected := DocRef{
			Ref:   ref,
			Type:  DocRefTypeResource,
			Token: "aws:s3:bucket",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("ResourceRefWithSlashInToken", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3%2Fbucket:Bucket"
		expected := DocRef{
			Ref:   ref,
			Type:  DocRefTypeResource,
			Token: "aws:s3/bucket:Bucket",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("FunctionRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/functions/aws:ec2:getInstance"
		expected := DocRef{
			Ref:   ref,
			Type:  DocRefTypeFunction,
			Token: "aws:ec2:getInstance",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("TypeRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/types/aws:s3:BucketPolicy"
		expected := DocRef{
			Ref:   ref,
			Type:  DocRefTypeType,
			Token: "aws:s3:BucketPolicy",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidUnknownPropertyKind", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/unknown/acl"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidPropertiesOnly", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/properties"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidMissingPropertyName", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/properties/"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("ResourcePropertyRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/properties/acl"
		expected := DocRef{
			Ref:      ref,
			Type:     DocRefTypeResourceProperty,
			Token:    "aws:s3:bucket",
			Property: "acl",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("ResourceInputPropertyRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/inputProperties/acl"
		expected := DocRef{
			Ref:      ref,
			Type:     DocRefTypeResourceInputProperty,
			Token:    "aws:s3:bucket",
			Property: "acl",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("FunctionInputPropertyRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/functions/aws:ec2:getInstance/inputs/properties/instanceId"
		expected := DocRef{
			Ref:      ref,
			Type:     DocRefTypeFunctionInputProperty,
			Token:    "aws:ec2:getInstance",
			Property: "instanceId",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("FunctionOutputPropertyRef", func(t *testing.T) {
		t.Parallel()
		ref := "#/functions/aws:ec2:getInstance/outputs/properties/publicIp"
		expected := DocRef{
			Ref:      ref,
			Type:     DocRefTypeFunctionOutputProperty,
			Token:    "aws:ec2:getInstance",
			Property: "publicIp",
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidMissingHashPrefix", func(t *testing.T) {
		t.Parallel()
		ref := "/resources/aws:s3:bucket"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("InvalidSubProperty", func(t *testing.T) {
		t.Parallel()
		ref := "#/resources/aws:s3:bucket/properties/acl/invalid"
		expected := DocRef{
			Ref:  ref,
			Type: DocRefTypeUnknown,
		}
		result := parseDocRef(ref)
		assert.Equal(t, expected, result)
	})

	t.Run("NewResource", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeResource, "aws:s3:bucket", "")
		assert.Equal(t, "#/resources/aws:s3:bucket", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewFunction", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeFunction, "aws:ec2:getInstance", "")
		assert.Equal(t, "#/functions/aws:ec2:getInstance", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewType", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeType, "aws:s3:BucketPolicy", "")
		assert.Equal(t, "#/types/aws:s3:BucketPolicy", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewResourceProperty", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeResourceProperty, "aws:s3:bucket", "acl")
		assert.Equal(t, "#/resources/aws:s3:bucket/properties/acl", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewResourceInputProperty", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeResourceInputProperty, "aws:s3:bucket", "acl")
		assert.Equal(t, "#/resources/aws:s3:bucket/inputProperties/acl", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewFunctionInputProperty", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeFunctionInputProperty, "aws:ec2:getInstance", "instanceId")
		assert.Equal(t, "#/functions/aws:ec2:getInstance/inputs/properties/instanceId", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewFunctionOutputProperty", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeFunctionOutputProperty, "aws:ec2:getInstance", "publicIp")
		assert.Equal(t, "#/functions/aws:ec2:getInstance/outputs/properties/publicIp", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewTypeProperty", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeTypeProperty, "aws:s3:BucketPolicy", "policy")
		assert.Equal(t, "#/types/aws:s3:BucketPolicy/properties/policy", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})

	t.Run("NewResourceWithSlashInToken", func(t *testing.T) {
		t.Parallel()
		docRef := NewDocRef(DocRefTypeResource, "aws:s3/bucket:Bucket", "")
		assert.Equal(t, "#/resources/aws:s3%2Fbucket:Bucket", docRef.Ref)
		assert.Equal(t, parseDocRef(docRef.Ref), docRef)
	})
}
