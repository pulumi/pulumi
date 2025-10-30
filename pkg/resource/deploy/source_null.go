package deploy

import deploy "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"

// Deprecated: A NullSource with no project name.
var NullSource = deploy.NullSource

// NullSource is a source that never returns any resources.  This may be used in scenarios where the "new"
// version of the world is meant to be empty, either for testing purposes, or removal of an existing stack.
func NewNullSource(project tokens.PackageName) Source {
	return deploy.NewNullSource(project)
}

