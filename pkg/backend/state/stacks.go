// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package state

import (
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func CurrentStack(backend backend.Backend) (backend.Stack, error) {
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}

	stackName := w.Settings().Stack
	if stackName == "" {
		return nil, nil
	}

	ref, err := backend.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	return backend.GetStack(ref)
}

// SetCurrentStack changes the current stack to the given stack name.
func SetCurrentStack(name string) error {
	// Switch the current workspace to that stack.
	w, err := workspace.New()
	if err != nil {
		return err
	}

	w.Settings().Stack = name
	return w.Save()
}
