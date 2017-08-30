// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"io"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/resource/environment"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type Engine struct {
	Stdout      io.Writer
	Stderr      io.Writer
	snk         diag.Sink
	Environment EnvironmentProvider
}

func (e *Engine) Diag() diag.Sink {
	if e.snk == nil {
		e.InitDiag(diag.FormatOptions{})
	}

	return e.snk
}

func (e *Engine) InitDiag(opts diag.FormatOptions) {
	contract.Assertf(e.snk == nil, "Cannot initialize diagnostics sink more than once")

	// Force using our stdout and stderr
	opts.Stdout = e.Stdout
	opts.Stderr = e.Stderr

	e.snk = diag.DefaultSink(opts)
}

// EnvironmentProvider abstracts away retriving and storing environments
type EnvironmentProvider interface {
	// GetEnvironment returns the environment named by `name` or a non nil error
	GetEnvironment(name tokens.QName) (*deploy.Target, *deploy.Snapshot, *environment.Checkpoint, error)
	// SaveEnvironment saves an environment o be retrieved later by GetEnvironment
	SaveEnvironment(env *deploy.Target, snap *deploy.Snapshot) error
	// RemoveEnvironment removes an environment from the system
	RemoveEnvironment(env *deploy.Target) error
}
