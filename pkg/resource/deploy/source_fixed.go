// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

// NewFixedSource returns a valid planning source that is comprised of a list of pre-computed resource objects.
func NewFixedSource(ctx tokens.PackageName, resources []*resource.Goal) Source {
	return &fixedSource{ctx: ctx, resources: resources}
}

// A fixedSource just returns from a fixed set of resource states.
type fixedSource struct {
	ctx       tokens.PackageName
	resources []*resource.Goal
}

func (src *fixedSource) Close() error {
	return nil // nothing to do.
}

func (src *fixedSource) Pkg() tokens.PackageName {
	return src.ctx
}

func (src *fixedSource) Info() interface{} {
	return nil
}

func (src *fixedSource) Iterate() (SourceIterator, error) {
	return &fixedSourceIterator{
		src:     src,
		current: -1,
	}, nil
}

// fixedSourceIterator always returns nil, nil in response to Next, indicating that it is done.
type fixedSourceIterator struct {
	src     *fixedSource
	current int
}

func (iter *fixedSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *fixedSourceIterator) Next() (SourceGoal, error) {
	iter.current++
	if iter.current >= len(iter.src.resources) {
		return nil, nil
	}
	return &fixedSourceResource{goal: iter.src.resources[iter.current]}, nil
}

type fixedSourceResource struct {
	goal *resource.Goal
}

func (res *fixedSourceResource) Resource() *resource.Goal {
	return res.goal
}

func (res *fixedSourceResource) Done(state *resource.State) {
	// nothing to do.
}
