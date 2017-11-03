// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Callers must call Close on the resulting planContext once they have completed the associated planning operation
func (eng *Engine) planContextFromStack(name tokens.QName, pkgarg string) (*planContext, error) {
	contract.Require(name != tokens.QName(""), "name")

	// Read in the deployment information, bailing if an IO error occurs.
	target, err := eng.Targets.GetTarget(name)
	if err != nil {
		return nil, errors.Wrap(err, "could not read target information")
	}

	snapshot, err := eng.Snapshots.GetSnapshot(name)
	if err != nil {
		return nil, errors.Wrap(err, "could not read snapshot information")
	}

	contract.Assert(target != nil)

	// Create a root span for the operation
	tracingSpan := opentracing.StartSpan("pulumi-plan")

	return &planContext{
		Target:      target,
		Snapshot:    snapshot,
		PackageArg:  pkgarg,
		TracingSpan: tracingSpan,
	}, nil
}

type planContext struct {
	Target      *deploy.Target   // the target stack.
	Snapshot    *deploy.Snapshot // the stack's latest deployment snapshot
	PackageArg  string           // an optional path to a package to pass to the compiler
	TracingSpan opentracing.Span // An OpenTracing span to parent plan operations within.
}

func (ctx *planContext) Close() {
	ctx.TracingSpan.Finish()
}
