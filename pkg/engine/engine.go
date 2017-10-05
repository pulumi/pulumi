// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
)

type Engine struct {
	Environment EnvironmentProvider
}

// EnvironmentProvider abstracts away retriving and storing environments
type EnvironmentProvider interface {
	// GetEnvironment returns the environment named by `name` or a non nil error
	GetEnvironment(name tokens.QName) (*deploy.Target, *deploy.Snapshot, error)
	// SaveEnvironment saves an environment o be retrieved later by GetEnvironment
	SaveEnvironment(env *deploy.Target, snap *deploy.Snapshot) error
	// RemoveEnvironment removes an environment from the system
	RemoveEnvironment(env *deploy.Target) error
}
