package completion

import completion "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/completion"

// NewGenCompletionCmd returns a new command that, when run, generates a bash or zsh completion script for the CLI.
func NewGenCompletionCmd(root *cobra.Command) *cobra.Command {
	return completion.NewGenCompletionCmd(root)
}

