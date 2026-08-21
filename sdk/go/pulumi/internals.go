// Copyright 2016, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Functions in this file are exposed in pulumi/internals via go:linkname
func awaitWithContext(ctx context.Context, o Output) (any, bool, bool, []Resource, error) {
	value, known, secret, deps, err := internal.AwaitOutput(ctx, o)
	return value, known, secret, resourcesFromInternal(deps), err
}

// registerCallback registers fn with the context's callback server, starting the server if necessary.
// Exposed in pulumi/workflow via go:linkname.
//
//nolint:unused // linkname target
func registerCallback(ctx *Context, fn callbackFunction) (*pulumirpc.Callback, error) {
	err := func() error {
		ctx.state.callbacksLock.Lock()
		defer ctx.state.callbacksLock.Unlock()
		if ctx.state.callbacks == nil {
			c, err := newCallbackServer()
			if err != nil {
				return fmt.Errorf("creating callback server: %w", err)
			}
			ctx.state.callbacks = c
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	return ctx.state.callbacks.RegisterCallback(fn)
}

// marshalValue marshals a plain value or Input (awaiting Outputs) into a property value. Exposed in
// pulumi/workflow via go:linkname.
//
//nolint:unused // linkname target
func marshalValue(v any) (resource.PropertyValue, error) {
	pv, _, err := marshalInput(v, anyType)
	return pv, err
}
