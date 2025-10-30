package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

// GetStack gets a stack and the project name & description, or returns nil if the stack doesn't exist.
func GetStack(ctx context.Context, b backend.Backend, stack string, opts display.Options) (backend.Stack, string, string, error) {
	return newcmd.GetStack(ctx, b, stack, opts)
}

// PromptAndCreateStack creates and returns a new stack (prompting for the name as needed).
func PromptAndCreateStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend, prompt promptForValueFunc, stack string, root string, setCurrent bool, yes bool, opts display.Options, secretsProvider string, useRemoteConfig bool) (backend.Stack, error) {
	return newcmd.PromptAndCreateStack(ctx, sink, ws, b, prompt, stack, root, setCurrent, yes, opts, secretsProvider, useRemoteConfig)
}

