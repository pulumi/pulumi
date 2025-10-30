package main

import main "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi"

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() (*cobra.Command, func()) {
	return main.NewPulumiCmd()
}

