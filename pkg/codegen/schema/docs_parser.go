package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// Shortcode represents a shortcode element and its contents, e.g. `{{% examples %}}`.
type Shortcode = schema.Shortcode

const ExamplesShortcode = schema.ExamplesShortcode

const ExampleShortcode = schema.ExampleShortcode

// KindShortcode is an ast.NodeKind for the Shortcode node.
var KindShortcode = schema.KindShortcode

// NewShortcode creates a new shortcode with the given name.
func NewShortcode(name []byte) *Shortcode {
	return schema.NewShortcode(name)
}

// NewShortcodeParser returns a BlockParser that parses shortcode (e.g. `{{% examples %}}`).
func NewShortcodeParser() parser.BlockParser {
	return schema.NewShortcodeParser()
}

// ParseDocs parses the given documentation text as Markdown with shortcodes and returns the AST.
func ParseDocs(docs []byte) ast.Node {
	return schema.ParseDocs(docs)
}

