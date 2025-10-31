package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

// HandleConfig handles prompting for config values (as needed) and saving config.
func HandleConfig(ctx context.Context, sink diag.Sink, ssml cmdStack.SecretsManagerLoader, ws pkgWorkspace.Context, prompt promptForValueFunc, project *workspace.Project, s backend.Stack, templateNameOrURL string, template workspace.Template, configArray []string, yes bool, path bool, opts display.Options) error {
	return newcmd.HandleConfig(ctx, sink, ssml, ws, prompt, project, s, templateNameOrURL, template, configArray, yes, path, opts)
}

// ParseConfig parses the config values passed via command line flags.
// These are passed as `-c aws:region=us-east-1 -c foo:bar=blah` and end up
// in configArray as ["aws:region=us-east-1", "foo:bar=blah"].
// This function converts the array into a config.Map.
func ParseConfig(configArray []string, path bool) (config.Map, error) {
	return newcmd.ParseConfig(configArray, path)
}

// SaveConfig saves the config for the stack.
func SaveConfig(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, stack backend.Stack, c config.Map) error {
	return newcmd.SaveConfig(ctx, sink, ws, stack, c)
}

