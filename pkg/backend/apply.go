package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// ApplierOptions is a bag of configuration settings for an Applier.
type ApplierOptions = backend.ApplierOptions

// Applier applies the changes specified by this update operation against the target stack.
type Applier = backend.Applier

// Explainer provides a contract for explaining changes that will be made to the stack.
// For Pulumi Cloud, this is a Copilot explainer.
type Explainer = backend.Explainer

func ActionLabel(kind apitype.UpdateKind, dryRun bool) string {
	return backend.ActionLabel(kind, dryRun)
}

func PreviewThenPrompt(ctx context.Context, kind apitype.UpdateKind, stack Stack, op UpdateOperation, apply Applier, explainer Explainer) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	return backend.PreviewThenPrompt(ctx, kind, stack, op, apply, explainer)
}

func PreviewThenPromptThenExecute(ctx context.Context, kind apitype.UpdateKind, stack Stack, op UpdateOperation, apply Applier, explainer Explainer, events chan<- engine.Event) (sdkDisplay.ResourceChanges, error) {
	return backend.PreviewThenPromptThenExecute(ctx, kind, stack, op, apply, explainer, events)
}

