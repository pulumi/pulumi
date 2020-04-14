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
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
)

var (
	// IMPORTANT! The following regexp's contain named capturing groups.
	// It's the `?P<group_name>` where group_name can be any name.
	// When changing the group names, be sure to change the reference to
	// the corresponding group name below where they are used as well.
	surroundingTextRE = regexp.MustCompile("({{% examples %}}(.|\n)*?{{% /examples %}})")
	examplesSectionRE = regexp.MustCompile(
		"(?P<examples_start>{{% examples %}})(?P<examples_content>(.|\n)*?)(?P<examples_end>{{% /examples %}})")
	individualExampleRE = regexp.MustCompile(
		"(?P<example_start>{{% example %}})(?P<example_content>(.|\n)*?)(?P<example_end>{{% /example %}})")
	h3TitleRE = regexp.MustCompile("(### .*)")

	// The following regexp's match the code snippet blocks in a single example section.
	tsCodeSnippetRE     = regexp.MustCompile("(```(typescript))((.|\n)*?)(```)")
	goCodeSnippetRE     = regexp.MustCompile("(```(go))((.|\n)*?)(```)")
	pythonCodeSnippetRE = regexp.MustCompile("(```(python))((.|\n)*?)(```)")
	csharpCodeSnippetRE = regexp.MustCompile("(```(csharp))((.|\n)*?)(```)")
)

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper interface {
	GetPropertyName(p *schema.Property) (string, error)
	GetDocLinkForResourceType(pkg *schema.Package, moduleName, typeName string) string
	GetDocLinkForPulumiType(pkg *schema.Package, typeName string) string
	GetDocLinkForResourceInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetDocLinkForFunctionInputOrOutputType(pkg *schema.Package, moduleName, typeName string, input bool) string
	GetDocLinkForBuiltInType(typeName string) string
	GetLanguageTypeString(pkg *schema.Package, moduleName string, t schema.Type, input, optional bool) string
	// GetResourceFunctionResultName returns the name of the result type when a static resource function is used to lookup
	// an existing resource.
	GetResourceFunctionResultName(resourceName string) string
}

type exampleParts struct {
	Title   string
	Snippet string
}

func getFirstMatchedGroupsFromRegex(regex *regexp.Regexp, str string) map[string]string {
	groups := map[string]string{}

	// Get all matching groups.
	matches := regex.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return groups
	}

	firstMatch := matches[0]
	// Get the named groups in our regex.
	groupNames := regex.SubexpNames()

	for i, value := range firstMatch {
		groups[groupNames[i]] = value
	}

	return groups
}

func getAllMatchedGroupsFromRegex(regex *regexp.Regexp, str string) map[string][]string {
	// Get all matching groups.
	matches := regex.FindAllStringSubmatch(str, -1)
	// Get the named groups in our regex.
	groupNames := regex.SubexpNames()

	groups := map[string][]string{}
	for _, match := range matches {
		for j, value := range match {
			if existing, ok := groups[groupNames[j]]; ok {
				existing = append(existing, value)
				groups[groupNames[j]] = existing
				continue
			}
			groups[groupNames[j]] = []string{value}
		}
	}

	return groups
}

func isEmpty(s string) bool {
	return strings.Replace(s, "\n", "", 1) == ""
}

// extractExamplesSection returns the content available between the {{% examples %}} shortcode.
// Otherwise returns nil.
func extractExamplesSection(description string) *string {
	examples := getFirstMatchedGroupsFromRegex(examplesSectionRE, description)
	if content, ok := examples["examples_content"]; ok && !isEmpty(content) {
		return &content
	}
	return nil
}

func identifyExampleParts(exampleContent string, lang string) *exampleParts {
	codeFence := "```" + lang
	langSnippetIndex := strings.Index(exampleContent, codeFence)
	// If there is no snippet for the provided language in this example,
	// then just return nil.
	if langSnippetIndex < 0 {
		return nil
	}

	var snippet string
	switch lang {
	case "csharp":
		snippet = csharpCodeSnippetRE.FindString(exampleContent)
	case "go":
		snippet = goCodeSnippetRE.FindString(exampleContent)
	case "python":
		snippet = pythonCodeSnippetRE.FindString(exampleContent)
	case "typescript":
		snippet = tsCodeSnippetRE.FindString(exampleContent)
	}

	return &exampleParts{
		Title:   h3TitleRE.FindString(exampleContent),
		Snippet: snippet,
	}
}

func getExamplesForLang(examplesContent string, lang string) []exampleParts {
	examples := make([]exampleParts, 0)
	exampleMatches := getAllMatchedGroupsFromRegex(individualExampleRE, examplesContent)
	if matchedExamples, ok := exampleMatches["example_content"]; ok {
		for _, ex := range matchedExamples {
			exampleParts := identifyExampleParts(ex, lang)
			if exampleParts == nil || exampleParts.Snippet == "" {
				continue
			}

			examples = append(examples, *exampleParts)
		}
	}
	return examples
}

// StripNonRelevantExamples strips the non-relevant language snippets from a resource's description.
func StripNonRelevantExamples(description string, lang string) string {
	if description == "" {
		return ""
	}

	// Replace the entire section (including the shortcodes themselves) enclosing the
	// examples section, with a placeholder, which itself will be replaced appropriately
	// later.
	newDescription := surroundingTextRE.ReplaceAllString(description, "{{ .Examples }}")

	// Get the content enclosing the outer examples short code.
	examplesContent := extractExamplesSection(description)
	if examplesContent == nil {
		return strings.ReplaceAll(newDescription, "{{ .Examples }}", "")
	}

	// Within the examples section, identify each example.
	builder := strings.Builder{}
	examples := getExamplesForLang(*examplesContent, lang)
	numExamples := len(examples)
	if numExamples > 0 {
		builder.WriteString("## Example Usage\n\n")
	}
	for i, ex := range examples {
		builder.WriteString(ex.Title + "\n\n")
		builder.WriteString(ex.Snippet + "\n")

		// Print an extra new-line character as long as this is not
		// the last example.
		if i != numExamples-1 {
			builder.WriteString("\n")
		}
	}

	return strings.ReplaceAll(newDescription, "{{ .Examples }}", builder.String())
}
