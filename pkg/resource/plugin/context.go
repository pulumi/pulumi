// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"context"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/resource"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	Diag      diag.Sink         // the diagnostics sink to use for messages.
	Host      Host              // the host that can be used to fetch providers.
	E         eval.Interpreter  // the interpreter shared amongst all planning in this context.
	ObjRes    objectResourceMap // the resources held inside of this snapshot.
	ObjURN    objectURNMap      // a convenient lookup map for object to URN.
	IDURN     idURNMap          // a convenient lookup map for ID to URN.
	URNRes    urnResourceMap    // a convenient lookup map for URN to resource.
	URNOldIDs urnIDMap          // a convenient lookup map for URNs to old IDs.
}

type objectURNMap map[*rt.Object]resource.URN
type objectResourceMap map[*rt.Object]resource.Resource
type idURNMap map[resource.ID]resource.URN
type urnResourceMap map[resource.URN]resource.Resource
type urnIDMap map[resource.URN]resource.ID

// NewContext allocates a new context with a given sink and host.  Note that the host is "owned" by this context from
// here forwards, such that when the context's resources are reclaimed, so too are the host's.
func NewContext(d diag.Sink, host Host) (*Context, error) {
	ctx := &Context{
		Diag:      d,
		Host:      host,
		ObjRes:    make(objectResourceMap),
		ObjURN:    make(objectURNMap),
		IDURN:     make(idURNMap),
		URNRes:    make(urnResourceMap),
		URNOldIDs: make(urnIDMap),
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
	// TODO[pulumi/lumi#143]: support cancellation.
	return context.TODO()
}

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	return ctx.Host.Close()
}
