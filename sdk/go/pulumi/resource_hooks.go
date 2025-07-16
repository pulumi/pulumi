// Copyright 2016-2021, Pulumi Corporation.
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
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/net/context"
)

// marshalResourceHooks marshals a `ResourceHookBinding` to a protobuf message.
// It will await any pending resource hook registrations.
func marshalResourceHooks(
	ctx context.Context, binding *ResourceHookBinding,
) (*pulumirpc.RegisterResourceRequest_ResourceHooksBinding, error) {
	hooks := &pulumirpc.RegisterResourceRequest_ResourceHooksBinding{}
	hooksValue := reflect.ValueOf(binding).Elem()
	protoHooksValue := reflect.ValueOf(hooks).Elem()
	hookFieldNames := []string{
		"BeforeCreate",
		"AfterCreate",
		"BeforeUpdate",
		"AfterUpdate",
		"BeforeDelete",
		"AfterDelete",
	}
	for _, fieldName := range hookFieldNames {
		hookSliceField := hooksValue.FieldByName(fieldName)
		if !hookSliceField.IsValid() || hookSliceField.IsNil() {
			continue
		}
		protoField := protoHooksValue.FieldByName(fieldName)
		if !protoField.IsValid() || !protoField.CanSet() {
			continue
		}
		for i := range hookSliceField.Len() {
			hook := hookSliceField.Index(i)
			if hook.IsNil() {
				continue
			}
			// Wait for the hook registration to complete.
			hookPtr := hook.Interface().(*ResourceHook)
			if _, err := hookPtr.registered.Result(ctx); err != nil {
				return nil, err
			}
			hookName := reflect.ValueOf(hookPtr.Name)
			protoField.Set(reflect.Append(protoField, hookName))
		}
	}
	return hooks, nil
}

// makeStubHooks creates a slice of `ResourceHooks` from hook names.
//
// We need to reconstruct `ResourceHook` instances to set on the
// `ResourceOption`, but we only have the names available to us. We also know
// that these hooks have already been registered when, so we can construct dummy
// hooks here, that will be serialized back into list of hook names.
func makeStubHooks(names []string) []*ResourceHook {
	// Create a fulfilled promise to mark the stubs as registered.
	c := promise.CompletionSource[struct{}]{}
	c.Fulfill(struct{}{})
	registered := c.Promise()
	stubHook := func(names []string) []*ResourceHook {
		hooks := []*ResourceHook{}
		for _, name := range names {
			hooks = append(hooks, &ResourceHook{
				Name:       name,
				registered: registered, // mark the stub hook as registered
			})
		}
		return hooks
	}
	return stubHook(names)
}
