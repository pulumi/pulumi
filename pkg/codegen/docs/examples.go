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
	"fmt"
	"strings"

	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const defaultMissingExampleSnippetPlaceholder = "Coming soon!"

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

func decomposeDocstring(docstring string) docInfo {
	if docstring == "" {
		return docInfo{}
	}

	languages := codegen.NewStringSet(snippetLanguages...)

	source := []byte(docstring)
	parsed := schema.ParseDocs(source)

	var examplesShortcode *schema.Shortcode
	var exampleShortcode *schema.Shortcode
	var title string
	var snippets map[string]string
	var examples []exampleSection
	err := ast.Walk(parsed, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if shortcode, ok := n.(*schema.Shortcode); ok {
			name := string(shortcode.Name)
			switch name {
			case schema.ExamplesShortcode:
				if examplesShortcode == nil {
					examplesShortcode = shortcode
				}
			case schema.ExampleShortcode:
				if exampleShortcode == nil {
					exampleShortcode, title, snippets = shortcode, "", map[string]string{}
				} else if !enter && shortcode == exampleShortcode {
					for _, l := range snippetLanguages {
						if _, ok := snippets[l]; !ok {
							snippets[l] = defaultMissingExampleSnippetPlaceholder
						}
					}

					examples = append(examples, exampleSection{
						Title:    title,
						Snippets: snippets,
					})

					exampleShortcode = nil
				}
			}
			return ast.WalkContinue, nil
		}
		if exampleShortcode == nil {
			return ast.WalkContinue, nil
		}

		switch n := n.(type) {
		case *ast.Heading:
			if n.Level == 3 && title == "" {
				title = strings.TrimSpace(schema.RenderDocsToString(source, n))
			}
		case *ast.FencedCodeBlock:
			language := string(n.Language(source))
			if !languages.Has(language) {
				return ast.WalkContinue, nil
			}
			if _, ok := snippets[language]; ok {
				return ast.WalkContinue, nil
			}

			snippet := schema.RenderDocsToString(source, n)
			snippets[language] = snippet
		}

		return ast.WalkContinue, nil
	})
	contract.AssertNoError(err)

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
	description = fmt.Sprintf("%s\n", parts[0])

	return docInfo{
		description:   description,
		examples:      examples,
		importDetails: importDetails,
	}
}
