package markdown

import markdown "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/markdown"

// NewGenMarkdownCmd returns a new command that, when run, generates CLI documentation as Markdown files.
// It is hidden by default since it's not commonly used outside of our own build processes.
func NewGenMarkdownCmd(root *cobra.Command) *cobra.Command {
	return markdown.NewGenMarkdownCmd(root)
}

