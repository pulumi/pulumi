using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi.X.Automation 
{
    /// <summary>
    /// Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
    /// <para/>
    /// Workspaces are used to manage the execution environment, providing various utilities such as plugin
    /// installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    /// </summary>
    public interface IWorkspace 
    {
        /// <summary>
        /// The working directory to run Pulumi CLI commands.
        /// </summary>
        string WorkDir { get; }

        /// <summary>as
        /// The directory override for CLI metadata if set.
        /// <para/>
        /// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
        /// </summary>
        string? PulumiHome { get; }

        /// <summary>
        /// The secrets provider to use for encryption and decryption of stack secrets.
        /// <para/>
        /// See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
        /// </summary>
        string? SecretsProvider { get; }

        /// <summary>
        /// The inline program <see cref="PulumiFn"/> to be used for Preview/Update operations if any.
        /// <para/>
        /// If none is specified, the stack will refer to <see cref="ProjectSettings"/> for this information.
        /// </summary>
        PulumiFn? Program { get; set; }

        /// <summary>
        /// Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
        /// </summary>
        IDictionary<string, string>? EnvironmentVariables { get; set;}

        /// <summary>
        /// Returns project settings for the current project if any.
        /// </summary>
        Task<ProjectSettings?> GetProjectSettingsAsync();

        /// <summary>
        /// Overwrites the settings for the current project.
        /// <para/>
        /// There can only be a single project per workspace. Fails if new project name does not match old.
        /// </summary>
        /// <param name="settings">The settings object to save.</param>
        Task SaveProjectSettingsAsync(ProjectSettings settings);

        /// <summary>
        /// Returns stack settings for the stack matching the specified stack name if any.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        Task<StackSettings?> GetStackSettingsAsync(string stackName);

        /// <summary>
        /// Overwrite the settings for the stack matching the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="settings">The settings object to save.</param>
        Task SaveStackSettingsAsync(string stackName, StackSettings settings);

        /// <summary>
        /// Hook to provide additional args to every CLI command before they are executed.
        /// <para/>
        /// Provided with a stack name, returns an array of args to append to an invoked command <c>["--config=...", ]</c>.
        /// <para/>
        /// <see cref="LocalWorkspace"/> does not utilize this extensibility point.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        Task<ImmutableList<string>> SerializeArgsForOpAsync(string stackName);

        /// <summary>
        /// Hook executed after every command. Called with the stack name.
        /// <para/>
        /// An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml).
        /// <para/>
        /// <see cref="LocalWorkspace"/> does not utilize this extensibility point.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        Task PostCommandCallbackAsync(string stackName);

        /// <summary>
        /// Returns the value associated with the specified stack name and key, scoped
        /// to the Workspace.
        /// </summary>
        /// <param name="stackName">The name of the stack to read config from.</param>
        /// <param name="key">The key to use for the config lookup.</param>
        Task<ConfigValue> GetConfigAsync(string stackName, string key);

        /// <summary>
        /// Returns the config map for the specified stack name, scoped to the current Workspace.
        /// </summary>
        /// <param name="stackName">The name of the stack to read config from.</param>
        Task<ImmutableDictionary<string, ConfigValue>> GetAllConfigAsync(string stackName);

        /// <summary>
        /// Sets the specified key-value pair in the provided stack's config.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="key">The config key to set.</param>
        /// <param name="value">The config value to set.</param>
        Task SetConfigAsync(string stackName, string key, ConfigValue value);

        /// <summary>
        /// Sets all values in the provided config map for the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="configMap">The config map to upsert against the existing config.</param>
        Task SetAllConfigAsync(string stackName, IDictionary<string, ConfigValue> configMap);

        /// <summary>
        /// Removes the specified key-value pair from the provided stack's config.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="key">The config key to remove.</param>
        Task RemoveConfigAsync(string stackName, string key);

        /// <summary>
        /// Removes all values in the provided key collection from the config map for the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="keys">The collection of keys to remove from the underlying config map.</param>
        Task RemoveAllConfigAsync(string stackName, IEnumerable<string> keys);

        /// <summary>
        /// Gets and sets the config map used with the last update for the stack matching the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(string stackName);

        /// <summary>
        /// Returns the currently authenticated user.
        /// </summary>
        Task<WhoAmIResult> WhoAmIAsync();

        /// <summary>
        /// Returns a summary of the currently selected stack, if any.
        /// </summary>
        Task<StackInfo?> GetStackAsync();

        /// <summary>
        /// Creates and sets a new stack with the specified stack name, failing if one already exists.
        /// </summary>
        /// <param name="stackName">The stack to create.</param>
        Task CreateStackAsync(string stackName);

        /// <summary>
        /// Selects and sets an existing stack matching the stack name, failing if none exists.
        /// </summary>
        /// <param name="stackName">The stack to select.</param>
        Task SelectStackAsync(string stackName);

        /// <summary>
        /// Deletes the stack and all associated configuration and history.
        /// </summary>
        /// <param name="stackName">The stack to remove.</param>
        Task RemoveStackAsync(string stackName);

        /// <summary>
        /// Returns all stacks created under the current project.
        /// <para/>
        /// This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.{stack}.yaml files).
        /// </summary>
        /// <returns></returns>
        Task<ImmutableList<StackInfo>> ListStacksAsync();

        /// <summary>
        /// Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.
        /// </summary>
        /// <param name="name">The name of the plugin.</param>
        /// <param name="version">The version of the plugin e.g. "v1.0.0".</param>
        /// <param name="kind">The kind of plugin e.g. "resource".</param>
        Task InstallPluginAsync(string name, string version, string? kind = null);

        /// <summary>
        /// Removes a plugin from the Workspace matching the specified name and version.
        /// </summary>
        /// <param name="name">The optional name of the plugin.</param>
        /// <param name="versionRange">The optional semver range to check when removing plugins matching the given name e.g. "1.0.0", ">1.0.0".</param>
        /// <param name="kind">The kind of plugin e.g. "resource".</param>
        Task RemovePluginAsync(string? name = null, string? versionRange = null, string? kind = null);

        /// <summary>
        /// Returns a list of all plugins installed in the Workspace.
        /// </summary>
        Task<ImmutableList<PluginInfo>> ListPluginsAsync();
    }
}
