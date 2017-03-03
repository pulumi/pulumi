// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"context"

	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/eval/rt"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	Diag      diag.Sink                  // the diagnostics sink to use for messages.
	Plugins   map[tokens.Package]*Plugin // a cache of plugins and their processes.
	ObjRes    objectResourceMap          // the resources held inside of this snapshot.
	ObjURN    objectURNMap               // a convenient lookup map for object to urn.
	URNRes    urnResourceMap             // a convenient lookup map for urn to resource.
	URNOldIDs urnIDMap                   // a convenient lookup map for urns to old IDs.
}

type objectURNMap map[*rt.Object]URN
type objectResourceMap map[*rt.Object]Resource
type urnResourceMap map[URN]Resource
type urnIDMap map[URN]ID

func NewContext(d diag.Sink) *Context {
	return &Context{
		Diag:      d,
		Plugins:   make(map[tokens.Package]*Plugin),
		ObjRes:    make(objectResourceMap),
		ObjURN:    make(objectURNMap),
		URNRes:    make(urnResourceMap),
		URNOldIDs: make(urnIDMap),
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
