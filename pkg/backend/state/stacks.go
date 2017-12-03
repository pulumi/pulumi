// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
		stacks, err := be.ListStacks()
		if err != nil {
			return nil, err
		}

		for _, stack := range stacks {
			if stack.Name() == name {
				return stack, nil
			}
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
