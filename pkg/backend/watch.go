package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// Watch watches the project's working directory for changes and automatically updates the active
// stack.
func Watch(ctx context.Context, b Backend, stack Stack, op UpdateOperation, apply Applier, paths []string) error {
	return backend.Watch(ctx, b, stack, op, apply, paths)
}

