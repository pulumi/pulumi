package auto

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Workspace TODO docstring
// TODO: many maybe all of these methods need to accept context objects to make sure they are extensible over network
type Workspace interface {
	// ProjectSettings returns the settings object for the current project if any
	ProjectSettings(context.Context) (*workspace.Project, error)
	// WriteProjectSettings overwrites the settings object in the current project.
	// There can only be a single project per workspace. Fails is new project name does not match old.
	WriteProjectSettings(context.Context, *workspace.Project) error
	// StackSettings returns the settings object for the stack matching the specified fullyQualifiedStackName if any.
	StackSettings(context.Context, string) (*workspace.ProjectStack, error)
	// WriteStackSettings overwrites the settings object for the stack matching the specified fullyQualifiedStackName.
	WriteStackSettings(context.Context, string, *workspace.ProjectStack) error
	// SerializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
	// Provided with fullyQualifiedStackName,
	// returns a list of args to append to an invoked command ["--config=...", ]
	SerializeArgsForOp(context.Context, string) ([]string, error)
	// PostOpCallback is a hook executed after every command. Called with the fullyQualifiedStackName.
	// An extensibility point to perform workspace cleanup (CLI operations may create/modify a pulumi.stack.yaml)
	PostOpCallback(context.Context, string) error
	// GetConfig returns the value associated with the specified fullyQualifiedStackName and key,
	// scoped to the current workspace.
	GetConfig(context.Context, string, string) (ConfigValue, error)
	// GetAllConfig returns the config map for the specified fullyQualifiedStackName, scoped to the current workspace.
	GetAllConfig(context.Context, string) (ConfigMap, error)
	// SetConfig sets the specified KVP on the provided fullyQualifiedStackName.
	SetConfig(context.Context, string, string, ConfigValue) error
	// SetAllConfig sets all values in the provided config map for the specified fullyQualifiedStackName
	SetAllConfig(context.Context, string, ConfigMap) error
	// RemoveConfig removes the specified KVP on the provided fullyQualifiedStackName.
	RemoveConfig(context.Context, string, string) error
	// RemoveAllConfig removes all values in the provided key list for the specified fullyQualifiedStackName
	RemoveAllConfig(context.Context, string, []string) error
	// RefreshConfig gets and sets the config map used with the last Update for Stack matching fullyQualifiedStackName.
	RefreshConfig(context.Context, string) (ConfigMap, error)
	// WorkDir returns the working directory to run Pulumi CLI commands.
	WorkDir() string
	// PulumiHome returns the directory override for CLI metadata if set.
	PulumiHome() *string
	// WhoAmI returns the currently authenticated user
	WhoAmI(context.Context) (string, error)
	// Stack returns the currently selected stack if any.
	Stack(context.Context) (*StackSummary, error)
	// CreateStack creates and sets a new stack with the fullyQualifiedStackName, failing if one already exists.
	CreateStack(context.Context, string) error
	// SelectStack selects and sets an existing stack matching the fullyQualifiedStackName, failing if none exists.
	SelectStack(context.Context, string) error
	// RemoveStack deletes the stack and all associated configuration and history.
	RemoveStack(context.Context, string) error
	// ListStacks returns all Stacks created under the current Project.
	// This queries underlying backend and may return stacks not present in the Workspace.
	ListStacks(context.Context) ([]StackSummary, error)
	// InstallPlugin acquires the plugin matching the specified name and version
	InstallPlugin(context.Context, string, string) error
	// RemovePlugin deletes the plugin matching the specified name and verision
	RemovePlugin(context.Context, string, string) error
	// ListPlugins lists all installed plugins.
	ListPlugins(context.Context) ([]workspace.PluginInfo, error)
	// Program returns the program `pulumi.RunFunc` to be used for Preview/Update if any.
	// If none is specified, the stack will refer to Project Settings for this information.
	Program() pulumi.RunFunc
	// SetProgram sets the program associated with the Workspace to the specified `pulumi.RunFunc`
	SetProgram(pulumi.RunFunc)
}

type ConfigValue struct {
	Value  string
	Secret bool
}

type ConfigMap map[string]ConfigValue

const PulumiHomeEnv = "PULUMI_HOME"

type StackSummary struct {
	Name             string `json:"name"`
	Current          bool   `json:"current"`
	LastUpdate       string `json:"lastUpdate,omitempty"`
	UpdateInProgress bool   `json:"updateInProgress"`
	ResourceCount    *int   `json:"resourceCount,omitempty"`
	URL              string `json:"url,omitempty"`
}
