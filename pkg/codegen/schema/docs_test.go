// Copyright 2020-2024, Pulumi Corporation.
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
	"encoding/json"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/testutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/stretchr/testify/assert"
)

// Note to future engineers: keep each file tested as a single test, do not use `t.Run` in the inner
// loops.
//
// Time to complete on these tests increases from ~2s to 30s or more and the number of lines logged
// to stdout from 46 lines to over 1,000,000 lines of output. This corresponds to the roughly 1
// million doc items tested across each file.
//
// Aside from just being verbose, the voluminous output makes `gotestsum` analysis less useful and
// prevents use of the `ci-matrix` tool.

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

var nodeAssertions = testutil.DefaultNodeAssertions().Union(testutil.NodeAssertions{
	KindShortcode: func(t *testing.T, sourceExpected, sourceActual []byte, expected, actual ast.Node) bool {
		shortcodeExpected, shortcodeActual := expected.(*Shortcode), actual.(*Shortcode)
		return testutil.AssertEqualBytes(t, shortcodeExpected.Name, shortcodeActual.Name)
	},
})

type doc struct {
	entity  string
	content string
}

func getDocsForProperty(parent string, name string, p PropertySpec) []doc {
	entity := path.Join(parent, name)
	return []doc{
		{entity: entity + "/description", content: p.Description},
		{entity: entity + "/deprecationMessage", content: p.DeprecationMessage},
	}
}

func getDocsForObjectType(path string, t *ObjectTypeSpec) []doc {
	if t == nil {
		return nil
	}

	docs := []doc{{entity: path + "/description", content: t.Description}}
	for name, p := range t.Properties {
		docs = append(docs, getDocsForProperty(path+"/properties", name, p)...)
	}
	return docs
}

func getDocsForFunction(token string, f FunctionSpec) []doc {
	entity := "#/functions/" + url.PathEscape(token)
	docs := []doc{
		{entity: entity + "/description", content: f.Description},
		{entity: entity + "/deprecationMessage", content: f.DeprecationMessage},
	}
	docs = append(docs, getDocsForObjectType(entity+"/inputs/properties", f.Inputs)...)

	if f.ReturnType != nil {
		if f.ReturnType.ObjectTypeSpec != nil {
			docs = append(docs, getDocsForObjectType(entity+"/outputs/properties", f.ReturnType.ObjectTypeSpec)...)
		}
	}

	return docs
}

func getDocsForResource(token string, r ResourceSpec, isProvider bool) []doc {
	var entity string
	if isProvider {
		entity = "#/provider"
	} else {
		entity = "#/resources/" + url.PathEscape(token)
	}

	docs := slice.Prealloc[doc](2 + len(r.InputProperties) + len(r.Properties))
	docs = append(docs,
		doc{entity: entity + "/description", content: r.Description},
		doc{entity: entity + "/deprecationMessage", content: r.DeprecationMessage},
	)
	for name, p := range r.InputProperties {
		docs = append(docs, getDocsForProperty(entity+"/inputProperties", name, p)...)
	}
	for name, p := range r.Properties {
		docs = append(docs, getDocsForProperty(entity+"/properties", name, p)...)
	}
	docs = append(docs, getDocsForObjectType(entity+"/stateInputs", r.StateInputs)...)
	return docs
}

func getDocsForPackage(pkg PackageSpec) []doc {
	var allDocs []doc
	for name, p := range pkg.Config.Variables {
		allDocs = append(allDocs, getDocsForProperty("#/config/variables", name, p)...)
	}
	for token, f := range pkg.Functions {
		allDocs = append(allDocs, getDocsForFunction(token, f)...)
	}
	allDocs = append(allDocs, getDocsForResource("", pkg.Provider, true)...)
	for token, r := range pkg.Resources {
		allDocs = append(allDocs, getDocsForResource(token, r, false)...)
	}
	for _, t := range pkg.Types {
		allDocs = append(allDocs, getDocsForObjectType("#/types", &t.ObjectTypeSpec)...)
	}
	return allDocs
}

func TestParseAndRenderDocs(t *testing.T) {
	files, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" || strings.Contains(f.Name(), "awsx") {
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

			path := filepath.Join(testdataPath, f.Name())
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read %v: %v", path, err)
			}

			var spec PackageSpec
			if err = json.Unmarshal(contents, &spec); err != nil {
				t.Fatalf("could not unmarshal package spec: %v", err)
			}

			//nolint:paralleltest // these are large, compute heavy tests. keep them in a single thread
			for _, doc := range getDocsForPackage(spec) {
				if doc.content == "" {
					continue
				}

				original := []byte(doc.content)
				expected := ParseDocumentation(string(original))
				rendered := []byte(expected.Render())
				actual := ParseDocumentation(string(rendered))
				if !testutil.AssertSameStructure(t, original, rendered, expected.node, actual.node, nodeAssertions) {
					t.Logf("original: %v", doc.content)
					t.Logf("rendered: %v", string(rendered))
				}
			}
		})
	}
}

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

		docs := ParseDocumentation(description)
		docs.FilterExamples("typescript")
		strippedDescription := docs.Render()
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, leadingDescription, "expected to at least find the leading description")
	})

	// The above description does not contain a Python code snippet and because
	// the description contains only one Example without any Python code snippet,
	// we should expect an empty string in this test.
	t.Run("DoesNotContainRelevantSnippet", func(t *testing.T) {
		t.Parallel()

		docs := ParseDocumentation(description)
		docs.FilterExamples("python")
		strippedDescription := docs.Render()
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

		docs := ParseDocumentation(description)
		docs.FilterExamples("typescript")
		strippedDescription := docs.Render()
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.Contains(t, strippedDescription, "Example 2", "expected Example 2 section")
	})

	t.Run("SomeExamplesHaveRelevantCodeSnippet", func(t *testing.T) {
		t.Parallel()

		docs := ParseDocumentation(description)
		docs.FilterExamples("go")
		strippedDescription := docs.Render()
		assert.NotEmpty(t, strippedDescription, "content could not be extracted")
		assert.Contains(t, strippedDescription, "Example 1", "expected Example 1 section")
		assert.NotContains(t, strippedDescription, "Example 2",
			"unexpected Example 2 section. section should have been excluded")
	})
}
