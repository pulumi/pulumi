// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/tokens"
)

type stackSummary struct {
	Name tokens.QName
	// May be "n/a" for an undeployed stack.
	LastDeploy    string
	ResourceCount string
}

type StackCreationOptions struct {
	Cloud string
}

var errHasResources = errors.New("stack has existing resources and force was false")

type pulumiBackend interface {
	CreateStack(stackName tokens.QName, opts StackCreationOptions) error
	GetStacks() ([]stackSummary, error)
	RemoveStack(stackName tokens.QName, force bool) error

	Preview(stackName tokens.QName, debug bool, opts engine.PreviewOptions) error
	Update(stackName tokens.QName, debug bool, opts engine.DeployOptions) error
	Destroy(stackName tokens.QName, debug bool, opts engine.DestroyOptions) error
}
