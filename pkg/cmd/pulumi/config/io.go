package config

import config "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/config"

// Attempts to load configuration for the given stack.
func GetStackConfiguration(ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, stack backend.Stack, project *workspace.Project) (backend.StackConfiguration, secrets.Manager, error) {
	return config.GetStackConfiguration(ctx, sink, ssml, stack, project)
}

// GetStackConfigurationOrLatest attempts to load a current stack configuration
// using getStackConfiguration. If that fails due to not being run within a
// valid project, the latest configuration from the backend is returned. This is
// primarily for use in commands like `pulumi destroy`, where it is useful to be
// able to clean up a stack whose configuration has already been deleted as part
// of that cleanup.
func GetStackConfigurationOrLatest(ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, stack backend.Stack, project *workspace.Project) (backend.StackConfiguration, secrets.Manager, error) {
	return config.GetStackConfigurationOrLatest(ctx, sink, ssml, stack, project)
}

// ParseConfigKey converts a given key string to a config.Key.
// Depending on whether the key is a path or not, the same string can either
// be valid or not, and also parse to different keys. For example:
// foo.bar:buzz is a (namespace: foo.bar, key: buzz) if not path, and
// (namespace: <project-name>, key: foo.bar:buzz) if path.
func ParseConfigKey(ws pkgWorkspace.Context, key string, path bool) (config.Key, error) {
	return config.ParseConfigKey(ws, key, path)
}

func PrettyKey(k config.Key) string {
	return config.PrettyKey(k)
}

