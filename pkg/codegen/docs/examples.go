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
// nolint: lll, goconst
package docs

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/v2/codegen"
)

const defaultMissingExampleSnippetPlaceholder = "Coming soon!"

type exampleSection struct {
	Title string
	// Snippets is a map of language to its code snippet, if any.
	Snippets map[string]string
}

// extractExampleCodeSnippets returns a map of code snippets by language.
// For any language that was missing in the provided example content, a
// placeholder snippet is set.
func extractExampleCodeSnippets(exampleContent string) map[string]string {
	snippets := map[string]string{}
	for _, lang := range supportedLanguages {
		var snippet string
		if lang == "nodejs" {
			lang = "typescript"
		}
		codeFence := "```" + lang
		langSnippetIndex := strings.Index(exampleContent, codeFence)
		// If there is no snippet for the provided language in this example,
		// then use a placeholder text for it.
		if langSnippetIndex < 0 {
			snippets[lang] = defaultMissingExampleSnippetPlaceholder
			continue
		}

		switch lang {
		case "csharp":
			snippet = codegen.CSharpCodeSnippetRE.FindString(exampleContent)
		case "go":
			snippet = codegen.GoCodeSnippetRE.FindString(exampleContent)
		case "python":
			snippet = codegen.PythonCodeSnippetRE.FindString(exampleContent)
		case "typescript":
			snippet = codegen.TSCodeSnippetRE.FindString(exampleContent)
		}

		snippets[lang] = snippet
	}

	return snippets
}

// getExampleSections returns a slice of all example sections organized
// by title and the code snippets. Returns an empty slice if examples
// were not detected due to bad formatting or otherwise.
func getExampleSections(examplesContent string) []exampleSection {
	examples := make([]exampleSection, 0)
	exampleMatches := codegen.GetAllMatchedGroupsFromRegex(codegen.IndividualExampleRE, examplesContent)
	if matchedExamples, ok := exampleMatches["example_content"]; ok {
		for _, ex := range matchedExamples {
			snippets := extractExampleCodeSnippets(ex)
			if snippets == nil || len(snippets) == 0 {
				continue
			}

			examples = append(examples, exampleSection{
				Title:    codegen.H3TitleRE.FindString(ex),
				Snippets: snippets,
			})
		}
	}
	return examples
}

// processExamples extracts the examples section from a resource or a function description
// and individually wraps in {{% example lang %}} short-codes. It also adds placeholder
// short-codes for missing languages.
func processExamples(descriptionWithExamples string) ([]exampleSection, error) {
	if descriptionWithExamples == "" {
		return nil, nil
	}

	// Get the content enclosing the outer examples short code.
	examplesContent, ok := codegen.ExtractExamplesSection(descriptionWithExamples)
	if !ok {
		return nil, nil
	}

	// Within the examples section, identify each example section
	// which is wrapped in a {{% example %}} shortcode.
	return getExampleSections(examplesContent), nil
}
