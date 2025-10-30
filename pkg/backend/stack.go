package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

type StackConfigLocation = backend.StackConfigLocation

// Stack is used to manage stacks of resources against a pluggable backend.
type Stack = backend.Stack

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(ctx context.Context, s Stack, force, removeBackups bool) (bool, error) {
	return backend.RemoveStack(ctx, s, force, removeBackups)
}

// RenameStack renames the stack, or returns an error if it cannot.
func RenameStack(ctx context.Context, s Stack, newName tokens.QName) (StackReference, error) {
	return backend.RenameStack(ctx, s, newName)
}

// PreviewStack previews changes to this stack.
func PreviewStack(ctx context.Context, s Stack, op UpdateOperation, events chan<- engine.Event) (*deploy.Plan, display.ResourceChanges, error) {
	return backend.PreviewStack(ctx, s, op, events)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(ctx context.Context, s Stack, op UpdateOperation, events chan<- engine.Event) (display.ResourceChanges, error) {
	return backend.UpdateStack(ctx, s, op, events)
}

// ImportStack updates the target stack with the current workspace's contents (config and code).
func ImportStack(ctx context.Context, s Stack, op UpdateOperation, imports []deploy.Import) (display.ResourceChanges, error) {
	return backend.ImportStack(ctx, s, op, imports)
}

// RefreshStack refresh's the stack's state from the cloud provider.
func RefreshStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return backend.RefreshStack(ctx, s, op)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return backend.DestroyStack(ctx, s, op)
}

// WatchStack watches the projects working directory for changes and automatically updates the
// active stack.
func WatchStack(ctx context.Context, s Stack, op UpdateOperation, paths []string) error {
	return backend.WatchStack(ctx, s, op, paths)
}

// GetLatestConfiguration returns the configuration for the most recent deployment of the stack.
func GetLatestConfiguration(ctx context.Context, s Stack) (config.Map, error) {
	return backend.GetLatestConfiguration(ctx, s)
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(ctx context.Context, secretsProvider secrets.Provider, s Stack, cfg StackConfiguration, query operations.LogQuery) ([]operations.LogEntry, error) {
	return backend.GetStackLogs(ctx, secretsProvider, s, cfg, query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(ctx context.Context, s Stack) (*apitype.UntypedDeployment, error) {
	return backend.ExportStackDeployment(ctx, s)
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(ctx context.Context, s Stack, deployment *apitype.UntypedDeployment) error {
	return backend.ImportStackDeployment(ctx, s, deployment)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func UpdateStackTags(ctx context.Context, s Stack, tags map[apitype.StackTagName]string) error {
	return backend.UpdateStackTags(ctx, s, tags)
}

// GetMergedStackTags returns the stack's existing tags merged with fresh tags from the environment
// and Pulumi.yaml file.
func GetMergedStackTags(ctx context.Context, s Stack, root string, project *workspace.Project, cfg config.Map) (map[apitype.StackTagName]string, error) {
	return backend.GetMergedStackTags(ctx, s, root, project, cfg)
}

// GetEnvironmentTagsForCurrentStack returns the set of tags for the "current" stack, based on the environment
// and Pulumi.yaml file.
func GetEnvironmentTagsForCurrentStack(root string, project *workspace.Project, cfg config.Map) (map[apitype.StackTagName]string, error) {
	return backend.GetEnvironmentTagsForCurrentStack(root, project, cfg)
}

