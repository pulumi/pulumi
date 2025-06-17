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
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ResourceHookFunction is the shape of a resource hook.
type ResourceHookFunction func(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	inputs resource.PropertyMap,
	outputs resource.PropertyMap,
) error

// ResourceHook represents a resource hook with it's handler and options.
type ResourceHook struct {
	Name     string
	Handler  ResourceHookFunction
	OnDryRun bool
}

// ResourceHooks is a registry of all resource hooks provided by a program.
type ResourceHooks struct {
	resourceHooksLock sync.Mutex
	resourceHooks     map[string]ResourceHook
}

func NewResourceHooks(dialOptions DialOptions) *ResourceHooks {
	return &ResourceHooks{}
}

func (l *ResourceHooks) RegisterResourceHook(hook ResourceHook) error {
	if hook.Name == "" {
		return errors.New("resource hook name cannot be empty")
	}

	l.resourceHooksLock.Lock()
	defer l.resourceHooksLock.Unlock()

	if _, has := l.resourceHooks[hook.Name]; has {
		return fmt.Errorf("resource hook already registered for name %q", hook.Name)
	}

	if l.resourceHooks == nil {
		l.resourceHooks = make(map[string]ResourceHook)
	}
	l.resourceHooks[hook.Name] = hook

	return nil
}

func (l *ResourceHooks) GetResourceHook(name string) (ResourceHook, error) {
	l.resourceHooksLock.Lock()
	defer l.resourceHooksLock.Unlock()

	if hook, has := l.resourceHooks[name]; has {
		return hook, nil
	}

	return ResourceHook{}, fmt.Errorf("resource hook not registered for %s", name)
}
