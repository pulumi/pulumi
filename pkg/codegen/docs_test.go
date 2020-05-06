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

func TestExtractExamplesSection(t *testing.T) {
	t.Run("NonEmptyContent", func(t *testing.T) {
		expectedContent := `
something here

and here

.... and there

..some..more here..
`
		description := `{{% examples %}}` + expectedContent + `{{% /examples %}}`

		actualContent := extractExamplesSection(description)
		assert.NotNil(t, actualContent, "content could not be extracted")
		assert.Equal(t, expectedContent, *actualContent, "strings don't match")
	})

	t.Run("EmptyContent", func(t *testing.T) {
		description := `{{% examples %}}
{{% /examples %}}`

		actualContent := extractExamplesSection(description)
		assert.Nil(t, actualContent, "expected content to be nil")
	})
}

var codeFence = "```"

// getWrappedExample returns an example code snippet for a given language in
// the following format:
//
// {{% example lang %}}
// ### Title
// ...code snippet...
// {{% /example %}}
func getWrappedExample(title, lang, snippet string) string {
	return "{{% example " + lang + " %}}" + "\n" + "### " + title + "\n" + snippet + "\n" + "{{% /example %}}"
}

func TestStripNonRelevantExamples(t *testing.T) {
	exampleTitle := "Example 1"
	tsCodeSnippet := codeFence + `typescript
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
	exampleShortCode := getWrappedExample(exampleTitle, "typescript", tsCodeSnippet) + "\n" + getWrappedExample(exampleTitle, "go", goCodeSnippet)
	description := leadingDescription + `
{{% examples %}}` + exampleShortCode + `
{{% /examples %}}`

	t.Run("ContainsRelevantCodeSnippet", func(t *testing.T) {
		strippedDescription := StripNonRelevantExamples(description, "typescript")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, leadingDescription, "expected to at least find the leading description")
	})

	// The above description does not contain a Python code snippet and because
	// the description contains only one Example without any Python code snippet,
	// we should expect an empty string in this test.
	t.Run("DoesNotContainRelevantSnippet", func(t *testing.T) {
		strippedDescription := StripNonRelevantExamples(description, "python")
		assert.Contains(t, strippedDescription, leadingDescription, "expected to at least find the leading description")
		// Should not contain any examples sections.
		assert.NotContains(t, strippedDescription, "### ", "expected to not have any examples but found at least one")
	})
}

func TestTestStripNonRelevantExamplesFromMultipleExampleSections(t *testing.T) {
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

	example1Title := "Example 1"
	example1ShortCode := getWrappedExample(example1Title, "typescript", tsCodeSnippet) + getWrappedExample(example1Title, "go", goCodeSnippet)

	example2Title := "Example 2"
	example2ShortCode := getWrappedExample(example2Title, "typescript", tsCodeSnippet)
	description := `{{% examples %}}` + "\n" + example1ShortCode + "\n" + example2ShortCode + "\n" + `{{% /examples %}}`

	t.Run("EveryExampleHasRelevantCodeSnippet", func(t *testing.T) {
		strippedDescription := StripNonRelevantExamples(description, "typescript")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.Contains(t, strippedDescription, "Example 2", "expected Example 2 section")
	})

	t.Run("SomeExamplesHaveRelevantCodeSnippet", func(t *testing.T) {
		strippedDescription := StripNonRelevantExamples(description, "go")
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.NotContains(t, strippedDescription, "Example 2",
			"unexpected Example 2 section. section should have been excluded")
	})
}
