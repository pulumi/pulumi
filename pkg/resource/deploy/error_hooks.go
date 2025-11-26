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

	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// ErrorHookFunction is the shape of an error hook callback.
// Given the list of errors for this registration so far, should we retry?
type ErrorHookFunction func(
	ctx context.Context,
	urn resource.URN,
	id resource.ID,
	name string,
	typ tokens.Type,
	newInputs resource.PropertyMap,
	oldInputs resource.PropertyMap,
	newOutputs resource.PropertyMap,
	oldOutputs resource.PropertyMap,
	errors []error, // all errors that have occurred so far, with the most recent first
) (bool, error) // should we retry?

// ErrorHook represents an error hook with its (wrapped) callback.
type ErrorHook struct {
	Name     string            // The unique name of the hook.
	Callback ErrorHookFunction // The callback of the hook.
}

type ErrorHooks struct {
	errorHooks *gsync.Map[string, ErrorHook]
}

func NewErrorHooks(dialOptions DialOptions) *ErrorHooks {
	return &ErrorHooks{
		errorHooks: &gsync.Map[string, ErrorHook]{},
	}
}

func (l *ErrorHooks) RegisterErrorHook(hook ErrorHook) error {
	if hook.Name == "" {
		return errors.New("error hook name cannot be empty")
	}
	if _, has := l.errorHooks.Load(hook.Name); has {
		return fmt.Errorf("error hook already registered for name %q", hook.Name)
	}
	l.errorHooks.Store(hook.Name, hook)
	return nil
}

func (l *ErrorHooks) GetErrorHook(name string) (ErrorHook, error) {
	hook, has := l.errorHooks.Load(name)
	if !has {
		return ErrorHook{}, fmt.Errorf("error hook not registered for %s", name)
	}
	return hook, nil
}
