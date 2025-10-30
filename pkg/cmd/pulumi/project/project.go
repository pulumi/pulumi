package project

import project "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/project"

// NewProjectCmd creates a new command that manages Pulumi projects.
func NewProjectCmd() *cobra.Command {
	return project.NewProjectCmd()
}

