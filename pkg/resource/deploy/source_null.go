// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// NullSource is a singleton source that never returns any resources.  This may be used in scenarios where the "new"
// version of the world is meant to be empty, either for testing purposes, or removal of an existing stack.
var NullSource Source = &nullSource{}

// A nullSource never returns any resources.
type nullSource struct {
}

func (src *nullSource) Close() error {
	return nil // nothing to do.
}

func (src *nullSource) Project() tokens.PackageName {
	return ""
}

func (src *nullSource) Info() interface{} {
	return nil
}

func (src *nullSource) Iterate(opts Options) (SourceIterator, error) {
	return &nullSourceIterator{}, nil
}

// nullSourceIterator always returns nil, nil in response to Next, indicating that it is done.
type nullSourceIterator struct {
}

func (iter *nullSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *nullSourceIterator) Next() (SourceEvent, error) {
	return nil, nil // means "done"
}
