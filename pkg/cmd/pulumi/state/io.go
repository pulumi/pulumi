package state

import state "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/state"

func TotalStateEdit(ctx context.Context, s backend.Stack, showPrompt bool, opts display.Options, operation func(display.Options, *deploy.Snapshot) error, overridePromptMessage *string) error {
	return state.TotalStateEdit(ctx, s, showPrompt, opts, operation, overridePromptMessage)
}

