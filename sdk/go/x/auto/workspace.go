package auto

import (
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Workspace TODO docstring
// TODO: many maybe all of these methods need to accept context objects to make sure they are extensible over network
type Workspace interface {
	// ProjectSettings returns the settings object for the current project if any
	ProjectSettings() (*workspace.Project, error)
	// WriteProjectSettings overwrites the settings object in the current project.
	// There can only be a single project per workspace. Fails is new project name does not match old.
	WriteProjectSettings(*workspace.Project) error
	// StackSettings returns the settings object for the stack matching the specified fullyQualifiedStackName if any.
	StackSettings(string) (*workspace.ProjectStack, error)
	// WriteStackSettings overwrites the settings object for the stack matching the specified fullyQualifiedStackName.
	WriteStackSettings(string, *workspace.ProjectStack) error
	// SerializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
	// Provided with fullyQualifiedStackName,
	// returns a list of args to append to an invoked command ["--config=...", ]
	SerializeArgsForOp(string) ([]string, error)
	// PostOpCallback is a hook executed after every command. Called with the fullyQualifiedStackName.
	// An extensibility point to perform workspace cleanup (CLI operations may create/modify a pulumi.stack.yaml)
	PostOpCallback(string) error
	// GetConfig returns the value associated with the specified fullyQualifiedStackName and key,
	// scoped to the current workspace.
	GetConfig(string, string) (ConfigValue, error)
	// GetAllConfig returns the config map for the specified fullyQualifiedStackName, scoped to the current workspace.
	GetAllConfig(string) (ConfigMap, error)
	// SetConfig sets the specified KVP on the provided fullyQualifiedStackName.
	SetConfig(string, string, ConfigValue) error
	// SetAllConfig sets all values in the provided config map for the specified fullyQualifiedStackName
	SetAllConfig(string, ConfigMap) error
	// RemoveConfig removes the specified KVP on the provided fullyQualifiedStackName.
	RemoveConfig(string, string) error
	// RemoveAllConfig removes all values in the provided key list for the specified fullyQualifiedStackName
	RemoveAllConfig(string, []string) error
	// RefreshConfig gets and sets the config map used with the last Update for Stack matching fullyQualifiedStackName.
	RefreshConfig(string) (ConfigMap, error)
	// WorkDir returns the working directory to run Pulumi CLI commands.
	WorkDir() string
	// PulumiHome returns the directory override for CLI metadata if set.
	PulumiHome() *string
	// WhoAmI returns the currently authenticated user
	WhoAmI() (string, error)
	// Stack returns the currently selected stack if any.
	Stack() (*StackSummary, error)
	// CreateStack creates and sets a new stack with the fullyQualifiedStackName, failing if one already exists.
	CreateStack(string) error
	// SelectStack selects and sets an existing stack matching the fullyQualifiedStackName, failing if none exists.
	SelectStack(string) error
	// RemoveStack deletes the stack and all associated configuration and history.
	RemoveStack(string) error
	// ListStacks returns all Stacks created under the current Project.
	// This queries underlying backend and may return stacks not present in the Workspace.
	ListStacks() ([]StackSummary, error)
	// InstallPlugin acquires the plugin matching the specified name and version
	InstallPlugin(string, string) error
	// RemovePlugin deletes the plugin matching the specified name and verision
	RemovePlugin(string, string) error
	// ListPlugins lists all installed plugins.
	ListPlugins() ([]workspace.PluginInfo, error)
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
