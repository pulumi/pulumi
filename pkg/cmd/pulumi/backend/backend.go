package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/backend"

// BackendInstance is used to inject a backend mock from tests.
var BackendInstance = backend.BackendInstance

func IsDIYBackend(ws pkgWorkspace.Context, opts display.Options) (bool, error) {
	return backend.IsDIYBackend(ws, opts)
}

func NonInteractiveCurrentBackend(ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project) (backend.Backend, error) {
	return backend.NonInteractiveCurrentBackend(ctx, ws, lm, project)
}

func CurrentBackend(ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project, opts display.Options) (backend.Backend, error) {
	return backend.CurrentBackend(ctx, ws, lm, project, opts)
}

