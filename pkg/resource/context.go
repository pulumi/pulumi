// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"context"

	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	plugins map[tokens.Package]*Plugin // a cache of plugins and their processes.
}

func NewContext() *Context {
	return &Context{
		plugins: make(map[tokens.Package]*Plugin),
	}
}

// Provider fetches the provider for a given resource, possibly lazily allocating the plugins for it.  If a provider
// could not be found, or an error occurred while creating it, a non-nil error is returned.
func (ctx *Context) Provider(pkg tokens.Package) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := ctx.plugins[pkg]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewPlugin(ctx, pkg)
	if err == nil {
		ctx.plugins[pkg] = plug // memoize the result.
	}
	return plug, err
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	// TODO: support cancellation.
	return context.TODO()
}
