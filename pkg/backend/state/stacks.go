package state

import state "github.com/pulumi/pulumi/sdk/v3/pkg/backend/state"

// CurrentStack reads the current stack and returns an instance connected to its backend provider.
func CurrentStack(ctx context.Context, b backend.Backend) (backend.Stack, error) {
	return state.CurrentStack(ctx, b)
}

// SetCurrentStack changes the current stack to the given stack name.
func SetCurrentStack(name string) error {
	return state.SetCurrentStack(name)
}

