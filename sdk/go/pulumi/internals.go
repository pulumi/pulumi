package pulumi

import "context"

// Functions in this file are exposed in pulumi/internals via go:linkname
func awaitWithContext(ctx context.Context, o Output) (interface{}, bool, bool, []Resource, error) {
	value, known, secret, deps, err := o.getState().await(ctx)

	return value, known, secret, deps, err
}
