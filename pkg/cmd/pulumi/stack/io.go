package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/stack"

type LoadOption = stack.LoadOption

const LoadOnly = stack.LoadOnly

const OfferNew = stack.OfferNew

const SetCurrent = stack.SetCurrent

var ConfigFile = stack.ConfigFile

func LoadProjectStack(ctx context.Context, sink diag.Sink, project *workspace.Project, stack_ backend.Stack) (*workspace.ProjectStack, error) {
	return stack.LoadProjectStack(ctx, sink, project, stack_)
}

func SaveProjectStack(ctx context.Context, stack_ backend.Stack, ps *workspace.ProjectStack) error {
	return stack.SaveProjectStack(ctx, stack_, ps)
}

// RequireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func RequireStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, stackName string, lopt LoadOption, opts display.Options) (backend.Stack, error) {
	return stack.RequireStack(ctx, sink, ws, lm, stackName, lopt, opts)
}

// ChooseStack will prompt the user to choose amongst the full set of stacks in the given backend.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func ChooseStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend, lopt LoadOption, opts display.Options) (backend.Stack, error) {
	return stack.ChooseStack(ctx, sink, ws, b, lopt, opts)
}

// InitStack creates the stack.
func InitStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend, stackName string, root string, setCurrent bool, secretsProvider string, useRemoteConfig bool) (backend.Stack, error) {
	return stack.InitStack(ctx, sink, ws, b, stackName, root, setCurrent, secretsProvider, useRemoteConfig)
}

// CreateStack creates a stack with the given name, and optionally selects it as the current.
func CreateStack(ctx context.Context, sink diag.Sink, ws pkgWorkspace.Context, b backend.Backend, stackRef backend.StackReference, root string, teams []string, setCurrent bool, secretsProvider string, useRemoteConfig bool) (backend.Stack, error) {
	return stack.CreateStack(ctx, sink, ws, b, stackRef, root, teams, setCurrent, secretsProvider, useRemoteConfig)
}

func CopyEntireConfigMap(ctx context.Context, ssml SecretsManagerLoader, currentStack backend.Stack, currentProjectStack *workspace.ProjectStack, destinationStack backend.Stack, destinationProjectStack *workspace.ProjectStack) (bool, error) {
	return stack.CopyEntireConfigMap(ctx, ssml, currentStack, currentProjectStack, destinationStack, destinationProjectStack)
}

func SaveSnapshot(ctx context.Context, s backend.Stack, snapshot *deploy.Snapshot, force bool) error {
	return stack.SaveSnapshot(ctx, s, snapshot, force)
}

