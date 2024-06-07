package sdkgen

import (
	"strings"

	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func commentTypeDoc(comment string) string {
	filtered := filterExamples(comment, "typescript")
	sanitized := sanitizeComment(filtered)
	trimmed := strings.TrimSpace(sanitized)
	return trimmed
}

func sanitizeComment(comment string) string {
	return strings.ReplaceAll(comment, "*/", "*&#47;")
}

// TODO we might need to rewrite examples here to use renamed packages -- see
// #15979.

// filterExamples filters the code snippets in a schema docstring to include
// only those that target the given language.
func filterExamples(description string, lang string) string {
	if description == "" {
		return ""
	}

	source := []byte(description)
	parsed := schema.ParseDocs(source)
	filterNodeExamples(source, parsed, lang)
	return schema.RenderDocsToString(source, parsed)
}

func filterNodeExamples(source []byte, node ast.Node, lang string) {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		filterNodeExamples(source, c, lang)

		next = c.NextSibling()
		switch c := c.(type) {
		case *ast.FencedCodeBlock:
			sourceLang := string(c.Language(source))
			if sourceLang != lang && sourceLang != "sh" {
				node.RemoveChild(node, c)
			}
		case *schema.Shortcode:
			switch string(c.Name) {
			case schema.ExampleShortcode:
				hasCode := false
				for gc := c.FirstChild(); gc != nil; gc = gc.NextSibling() {
					if gc.Kind() == ast.KindFencedCodeBlock {
						hasCode = true
						break
					}
				}
				if hasCode {
					var grandchild, nextGrandchild ast.Node
					for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
						nextGrandchild = grandchild.NextSibling()
						node.InsertBefore(node, c, grandchild)
					}
				}
				node.RemoveChild(node, c)
			case schema.ExamplesShortcode:
				if first := c.FirstChild(); first != nil {
					first.SetBlankPreviousLines(c.HasBlankPreviousLines())
				}

				var grandchild, nextGrandchild ast.Node
				for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
					nextGrandchild = grandchild.NextSibling()
					node.InsertBefore(node, c, grandchild)
				}
				node.RemoveChild(node, c)
			}
		}
	}
}
