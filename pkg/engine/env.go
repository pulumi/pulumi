// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Callers must call Close on the resulting planContext once they have completed the associated planning operation
func planContextFromUpdate(update Update) (*planContext, error) {
	contract.Require(update != nil, "update")

	// Create a root span for the operation
	tracingSpan := opentracing.StartSpan("pulumi-plan")

	return &planContext{
		Update:      update,
		TracingSpan: tracingSpan,
	}, nil
}

type planContext struct {
	Update      Update           // The update being processed.
	TracingSpan opentracing.Span // An OpenTracing span to parent plan operations within.
}

func (ctx *planContext) Close() {
	ctx.TracingSpan.Finish()
}
