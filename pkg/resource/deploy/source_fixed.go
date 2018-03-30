// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// NewFixedSource returns a valid planning source that is comprised of a list of pre-computed steps.
func NewFixedSource(ctx tokens.PackageName, steps []SourceEvent) Source {
	return &fixedSource{ctx: ctx, steps: steps}
}

// A fixedSource just returns from a fixed set of resource states.
type fixedSource struct {
	ctx   tokens.PackageName
	steps []SourceEvent
}

func (src *fixedSource) Close() error {
	return nil // nothing to do.
}

func (src *fixedSource) Project() tokens.PackageName {
	return src.ctx
}

func (src *fixedSource) Info() interface{} {
	return nil
}

func (src *fixedSource) Iterate(opts Options) (SourceIterator, error) {
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

func (iter *fixedSourceIterator) Next() (SourceEvent, error) {
	iter.current++
	if iter.current >= len(iter.src.steps) {
		return nil, nil
	}
	return iter.src.steps[iter.current], nil
}
