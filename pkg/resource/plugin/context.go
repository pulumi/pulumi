// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"context"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/util/rpcutil"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	Diag diag.Sink // the diagnostics sink to use for messages.
	Host Host      // the host that can be used to fetch providers.
}

// NewContext allocates a new context with a given sink and host.  Note that the host is "owned" by this context from
// here forwards, such that when the context's resources are reclaimed, so too are the host's.
func NewContext(d diag.Sink, host Host) (*Context, error) {
	ctx := &Context{
		Diag: d,
		Host: host,
	}
	if host == nil {
		h, err := NewDefaultHost(ctx)
		if err != nil {
			return nil, err
		}
		ctx.Host = h
	}
	return ctx, nil
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	// TODO[pulumi/pulumi-fabric#143]: support cancellation.
	return context.TODO()
}

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	err := ctx.Host.Close()
	if err != nil && !rpcutil.IsBenignCloseErr(err) {
		return err
	}
	return nil
}
