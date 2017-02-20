// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"context"

	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	Plugins map[tokens.Package]*Plugin // a cache of plugins and their processes.
	Res     objectResourceMap          // the resources held inside of this snapshot.
	Mks     monikerResourceMap         // a convenient lookup map for moniker to resource.
}

type objectResourceMap map[*rt.Object]Resource
type monikerResourceMap map[Moniker]Resource

func NewContext() *Context {
	return &Context{
		Plugins: make(map[tokens.Package]*Plugin),
		Res:     make(objectResourceMap),
		Mks:     make(monikerResourceMap),
	}
}

// Provider fetches the provider for a given resource, possibly lazily allocating the plugins for it.  If a provider
// could not be found, or an error occurred while creating it, a non-nil error is returned.
func (ctx *Context) Provider(pkg tokens.Package) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := ctx.Plugins[pkg]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewPlugin(ctx, pkg)
	if err == nil {
		ctx.Plugins[pkg] = plug // memoize the result.
	}
	return plug, err
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	// TODO: support cancellation.
	return context.TODO()
}
