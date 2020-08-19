package auto

import (
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
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
	GetConfig(string, config.Key) (config.Value, error)
	// GetAllConfig returns the config map for the specified fullyQualifiedStackName, scoped to the current workspace.
	GetAllConfig(string) (config.Map, error)
	// SetConfig sets the specified KVP on the provided fullyQualifiedStackName.
	SetConfig(string, config.Key, config.Value) error
	// SetAllConfig overwrites the current config map for the specified fullyQualifiedStackName
	SetAllConfig(string, config.Map) error
	// RefreshConfig gets and sets the config map used with the last Update for Stack matching fullyQualifiedStackName.
	RefreshConfig(string) (config.Map, error)
	// WorkDir returns the working directory to run Pulumi CLI commands.
	WorkDir() string
	// PulumiHome returns the directory override for CLI metadata if set.
	PulumiHome() *string
	// Stack returns the fullyQualifiedStackName of the currently selected stack if any.
	Stack() string
	// CreateStack creates and sets a new stack with the fullyQualifiedStackName, failing if one already exists.
	CreateStack(string) (Stack, error)
	// SelectStack selects and sets an existing stack matching the fullyQualifiedStackName, failing if none exists.
	SelectStack(string) (Stack, error)
	// ListStacks returns the fullyQualifiedStackNames of all Stacks created under the current Project.
	// This queries underlying backend and may return stacks not present in the Workspace.
	ListStacks(string) ([]string, error)
	// InstallPlugin acquires the plugin matching the specified name and version
	InstallPlugin(string, string) error
	// RemovePlugin deletes the plugin matching the specified name and verision
	RemovePlugin(string, string) error
	// ListPlugins lists all installed plugins.
	ListPlugins() ([]workspace.PluginInfo, error)
}
