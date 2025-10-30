package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"

// A RendererOption controls the behavior of a Renderer.
type RendererOption = schema.RendererOption

// A ReferenceRenderer is responsible for rendering references to entities in a schema.
type ReferenceRenderer = schema.ReferenceRenderer

// A Renderer provides the ability to render parsed documentation back to Markdown source.
type Renderer = schema.Renderer

// WithReferenceRenderer sets the reference renderer for a renderer.
func WithReferenceRenderer(refRenderer ReferenceRenderer) RendererOption {
	return schema.WithReferenceRenderer(refRenderer)
}

// RenderDocs renders parsed documentation to the given Writer. The source that was used to parse the documentation
// must be provided.
func RenderDocs(w io.Writer, source []byte, node ast.Node, options ...RendererOption) error {
	return schema.RenderDocs(w, source, node, options...)
}

// RenderDocsToString is like RenderDocs, but renders to a string instead of a Writer.
func RenderDocsToString(source []byte, node ast.Node, options ...RendererOption) string {
	return schema.RenderDocsToString(source, node, options...)
}

