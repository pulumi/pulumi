// Copyright 2025, Pulumi Corporation.
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

package deploy

import (
	"context"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// LifecycleHookFunction is the shape of a lifecycle hook
// TODO: all args & make optional
type LifecycleHookFunction func(ctx context.Context, urn resource.URN, id resource.ID) error

// LifecycleHooks is a registry of all lifecycle hooks provided by a program.
type LifecycleHooks struct {
	lifecycleHooksLock sync.Mutex
	lifecycleHooks     map[string]LifecycleHookFunction
}

func NewLifecycleHooks(dialOptions DialOptions) *LifecycleHooks {
	return &LifecycleHooks{}
}

func (l *LifecycleHooks) RegisterLifecycleHook(name string, hook LifecycleHookFunction) error {
	l.lifecycleHooksLock.Lock()
	defer l.lifecycleHooksLock.Unlock()

	if _, has := l.lifecycleHooks[name]; has {
		return fmt.Errorf("lifecycle hook already registered for name %q", name)
	}

	if l.lifecycleHooks == nil {
		l.lifecycleHooks = make(map[string]LifecycleHookFunction)
	}
	l.lifecycleHooks[name] = hook

	return nil
}

func (l *LifecycleHooks) GetLifecycleHook(name string) (LifecycleHookFunction, error) {
	l.lifecycleHooksLock.Lock()
	defer l.lifecycleHooksLock.Unlock()

	if hook, has := l.lifecycleHooks[name]; has {
		return hook, nil
	}

	return nil, fmt.Errorf("lifecycle hook not registered for %s", name)
}
