package ai

import ai "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/ai"

func NewAICommand() *cobra.Command {
	return ai.NewAICommand()
}

