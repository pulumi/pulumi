// Copyright 2017-2018, Pulumi Corporation.
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

package state

import (
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack returns an stack name after ensuring the stack exists.  When an empty stack name is passed, the
// "current" ambient stack is returned.
func Stack(name tokens.QName, backends []backend.Backend) (backend.Stack, error) {
	if name == "" {
		return CurrentStack(backends)
	}

	// If not using the current stack, check all of the known backends to see if they know about it.
	for _, be := range backends {
		stack, err := be.GetStack(name)
		if err != nil {
			return nil, err
		}
		if stack != nil {
			return stack, nil
		}
	}

	return nil, nil
}

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func CurrentStack(backends []backend.Backend) (backend.Stack, error) {
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}

	stackName := w.Settings().Stack
	if stackName == "" {
		return nil, nil
	}

	return Stack(stackName, backends)
}

// SetCurrentStack changes the current stack to the given stack name.
func SetCurrentStack(name tokens.QName) error {
	// Switch the current workspace to that stack.
	w, err := workspace.New()
	if err != nil {
		return err
	}

	w.Settings().Stack = name
	return w.Save()
}
