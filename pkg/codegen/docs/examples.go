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
package docs

import (
	"strings"

	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const defaultMissingExampleSnippetPlaceholder = "Coming soon!"

type examplesSection struct {
	// Examples is a list of exampleSections. Each exampleSection contains a title and code snippets
	Examples []exampleSection
	// LangChooserLanguages is a comma-separated list of languages to pass to the
	// language chooser shortcode. Use this to customize the languages shown for a
	// resource. By default, the language chooser will show all languages supported
	// by Pulumi for all resources.
	// Supported values are "typescript", "python", "go", "csharp", "java", "yaml"
	LangChooserLanguages string
}

type exampleSection struct {
	Title string
	// Snippets is a map of language to its code snippet, if any.
	Snippets map[string]string
}

type docInfo struct {
	description   string
	examples      []exampleSection
	importDetails string
}

func (dctx *docGenContext) decomposeDocstring(docstring, supportedSnippetLanguages string) docInfo {
	if docstring == "" {
		return docInfo{}
	}
	if strings.Contains(docstring, beginCodeBlock) {
		return dctx.processDescription(docstring, supportedSnippetLanguages)
	}

	languages := codegen.NewStringSet(dctx.snippetLanguages...)

	source := []byte(docstring)
	parsed := schema.ParseDocs(source)

	var examplesShortcode *schema.Shortcode
	var exampleShortcode *schema.Shortcode
	var examples []exampleSection
	currentSection := exampleSection{
		Snippets: map[string]string{},
	}
	var nextTitle string
	var nextInferredTitle string
	// Push any examples we have found. Since `pushExamples` is called between sections,
	// it needs to behave correctly when no examples were found.
	pushExamples := func() {
		if len(currentSection.Snippets) > 0 {
			for _, l := range dctx.snippetLanguages {
				if _, ok := currentSection.Snippets[l]; !ok {
					currentSection.Snippets[l] = defaultMissingExampleSnippetPlaceholder
				}
			}

			examples = append(examples, currentSection)
		}
		if nextTitle == "" {
			nextTitle = nextInferredTitle
		}
		currentSection = exampleSection{
			Snippets: map[string]string{},
			Title:    nextTitle,
		}
		nextTitle = ""
		nextInferredTitle = ""
	}
	err := ast.Walk(parsed, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		// ast.Walk visits each node twice. The first time descending and the second time
		// ascending. We only want to view the nodes while descending, so we skip when
		// `enter` is false.
		if !enter {
			return ast.WalkContinue, nil
		}
		if shortcode, ok := n.(*schema.Shortcode); ok {
			name := string(shortcode.Name)
			switch name {
			case schema.ExamplesShortcode:
				if examplesShortcode == nil {
					examplesShortcode = shortcode
				}
			case schema.ExampleShortcode:
				if exampleShortcode == nil {
					exampleShortcode = shortcode
					currentSection.Title, currentSection.Snippets = "", map[string]string{}
				} else if !enter && shortcode == exampleShortcode {
					pushExamples()
					exampleShortcode = nil
				}
			}
			return ast.WalkContinue, nil
		}

		// We check to make sure we are in an examples section.
		if exampleShortcode == nil {
			return ast.WalkContinue, nil
		}

		switch n := n.(type) {
		case *ast.Heading:
			if n.Level == 3 {
				title := strings.TrimSpace(schema.RenderDocsToString(source, n))
				if currentSection.Title == "" && len(currentSection.Snippets) == 0 {
					currentSection.Title = title
				} else {
					nextTitle = title
				}
			}
			return ast.WalkSkipChildren, nil

		case *ast.FencedCodeBlock:
			language := string(n.Language(source))
			snippet := schema.RenderDocsToString(source, n)
			if !languages.Has(language) || len(snippet) == 0 {
				return ast.WalkContinue, nil
			}
			if _, ok := currentSection.Snippets[language]; ok {
				// We have the same language appearing multiple times in a {{% examples
				// %}} without an {{% example %}} to break them up. We are going to just
				// pretend there was an {{% example %}}
				pushExamples()
			}
			currentSection.Snippets[language] = snippet
		case *ast.Text:
			// We only want to change the title before we collect any snippets
			title := strings.TrimSuffix(string(n.Text(source)), ":")
			if currentSection.Title == "" && len(currentSection.Snippets) == 0 {
				currentSection.Title = title
			} else {
				// Since we might find out we are done with the previous section only
				// after we have consumed the next title, we store the title.
				nextInferredTitle = title
			}
		}

		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "error walking AST")
	pushExamples()

	if examplesShortcode != nil {
		p := examplesShortcode.Parent()
		p.RemoveChild(p, examplesShortcode)
	}

	description := schema.RenderDocsToString(source, parsed)
	importDetails := ""
	parts := strings.Split(description, "\n\n## Import")
	if len(parts) > 1 { // we only care about the Import section details here!!
		importDetails = parts[1]
	}

	// When we split the description above, the main part of the description is always part[0]
	// the description must have a blank line after it to render the examples correctly
	description = parts[0] + "\n"

	return docInfo{
		description:   description,
		examples:      examples,
		importDetails: importDetails,
	}
}
