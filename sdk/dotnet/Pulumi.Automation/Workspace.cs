// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Pulumi.Automation.Commands;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;
// ReSharper disable UnusedMemberInSuper.Global
// ReSharper disable VirtualMemberNeverOverridden.Global

namespace Pulumi.Automation
{
    /// <summary>
    /// Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
    /// <para/>
    /// Workspaces are used to manage the execution environment, providing various utilities such as plugin
    /// installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    /// </summary>
    public abstract class Workspace : IDisposable
    {
        private readonly IPulumiCmd _cmd;

        internal Workspace(IPulumiCmd cmd)
        {
            this._cmd = cmd;
        }

        /// <summary>
        /// The working directory to run Pulumi CLI commands.
        /// </summary>
        public abstract string WorkDir { get; }

        /// <summary>
        /// The directory override for CLI metadata if set.
        /// <para/>
        /// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
        /// </summary>
        public abstract string? PulumiHome { get; }

        /// <summary>
        /// The version of the underlying Pulumi CLI/Engine.
        /// </summary>
        public abstract string PulumiVersion { get; }

        /// <summary>
        /// The secrets provider to use for encryption and decryption of stack secrets.
        /// <para/>
        /// See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
        /// </summary>
        public abstract string? SecretsProvider { get; }

        /// <summary>
        /// The inline program <see cref="PulumiFn"/> to be used for Preview/Update operations if any.
        /// <para/>
        /// If none is specified, the stack will refer to <see cref="ProjectSettings"/> for this information.
        /// </summary>
        public abstract PulumiFn? Program { get; set; }

        /// <summary>
        /// A custom logger instance that will be used for the action. Note that it will only be used
        /// if <see cref="Program"/> is also provided.
        /// </summary>
        public abstract ILogger? Logger { get; set; }

        /// <summary>
        /// Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
        /// </summary>
        public abstract IDictionary<string, string?>? EnvironmentVariables { get; set; }

        /// <summary>
        /// Returns project settings for the current project if any.
        /// </summary>
        public abstract Task<ProjectSettings?> GetProjectSettingsAsync(CancellationToken cancellationToken = default);

        /// <summary>
        /// Overwrites the settings for the current project.
        /// <para/>
        /// There can only be a single project per workspace. Fails if new project name does not match old.
        /// </summary>
        /// <param name="settings">The settings object to save.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task SaveProjectSettingsAsync(ProjectSettings settings, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns stack settings for the stack matching the specified stack name if any.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<StackSettings?> GetStackSettingsAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Overwrite the settings for the stack matching the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="settings">The settings object to save.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task SaveStackSettingsAsync(string stackName, StackSettings settings, CancellationToken cancellationToken = default);

        /// <summary>
        /// Hook to provide additional args to every CLI command before they are executed.
        /// <para/>
        /// Provided with a stack name, returns an array of args to append to an invoked command <c>["--config=...", ]</c>.
        /// <para/>
        /// <see cref="LocalWorkspace"/> does not utilize this extensibility point.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<ImmutableList<string>> SerializeArgsForOpAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Hook executed after every command. Called with the stack name.
        /// <para/>
        /// An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml).
        /// <para/>
        /// <see cref="LocalWorkspace"/> does not utilize this extensibility point.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task PostCommandCallbackAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns the value associated with the specified stack name and key, scoped
        /// to the Workspace.
        /// </summary>
        /// <param name="stackName">The name of the stack to read config from.</param>
        /// <param name="key">The key to use for the config lookup.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<ConfigValue> GetConfigAsync(string stackName, string key, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns the config map for the specified stack name, scoped to the current Workspace.
        /// </summary>
        /// <param name="stackName">The name of the stack to read config from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<ImmutableDictionary<string, ConfigValue>> GetAllConfigAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Sets the specified key-value pair in the provided stack's config.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="key">The config key to set.</param>
        /// <param name="value">The config value to set.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task SetConfigAsync(string stackName, string key, ConfigValue value, CancellationToken cancellationToken = default);

        /// <summary>
        /// Sets all values in the provided config map for the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="configMap">The config map to upsert against the existing config.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task SetAllConfigAsync(string stackName, IDictionary<string, ConfigValue> configMap, CancellationToken cancellationToken = default);

        /// <summary>
        /// Removes the specified key-value pair from the provided stack's config.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="key">The config key to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task RemoveConfigAsync(string stackName, string key, CancellationToken cancellationToken = default);

        /// <summary>
        /// Removes all values in the provided key collection from the config map for the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="keys">The collection of keys to remove from the underlying config map.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task RemoveAllConfigAsync(string stackName, IEnumerable<string> keys, CancellationToken cancellationToken = default);

        /// <summary>
        /// Gets and sets the config map used with the last update for the stack matching the specified stack name.
        /// </summary>
        /// <param name="stackName">The name of the stack to operate on.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns the currently authenticated user.
        /// </summary>
        public abstract Task<WhoAmIResult> WhoAmIAsync(CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns a summary of the currently selected stack, if any.
        /// </summary>
        public virtual async Task<StackSummary?> GetStackAsync(CancellationToken cancellationToken = default)
        {
            var stacks = await this.ListStacksAsync(cancellationToken).ConfigureAwait(false);
            return stacks.FirstOrDefault(x => x.IsCurrent);
        }

        /// <summary>
        /// Creates and sets a new stack with the specified stack name, failing if one already exists.
        /// </summary>
        /// <param name="stackName">The stack to create.</param>
        public Task CreateStackAsync(string stackName)
            => this.CreateStackAsync(stackName, default);

        /// <summary>
        /// Creates and sets a new stack with the specified stack name, failing if one already exists.
        /// </summary>
        /// <param name="stackName">The stack to create.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        /// <exception cref="StackAlreadyExistsException">If a stack already exists by the provided name.</exception>
        public abstract Task CreateStackAsync(string stackName, CancellationToken cancellationToken);

        /// <summary>
        /// Selects and sets an existing stack matching the stack name, failing if none exists.
        /// </summary>
        /// <param name="stackName">The stack to select.</param>
        /// <exception cref="StackNotFoundException">If no stack was found by the provided name.</exception>
        public Task SelectStackAsync(string stackName)
            => this.SelectStackAsync(stackName, default);

        /// <summary>
        /// Selects and sets an existing stack matching the stack name, failing if none exists.
        /// </summary>
        /// <param name="stackName">The stack to select.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task SelectStackAsync(string stackName, CancellationToken cancellationToken);

        /// <summary>
        /// Deletes the stack and all associated configuration and history.
        /// </summary>
        /// <param name="stackName">The stack to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task RemoveStackAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns all stacks created under the current project.
        /// <para/>
        /// This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.{stack}.yaml files).
        /// </summary>
        public abstract Task<ImmutableList<StackSummary>> ListStacksAsync(CancellationToken cancellationToken = default);

        /// <summary>
        /// Exports the deployment state of the stack.
        /// <para/>
        /// This can be combined with ImportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public abstract Task<StackDeployment> ExportStackAsync(string stackName, CancellationToken cancellationToken = default);

        /// <summary>
        /// Imports the specified deployment state into a pre-existing stack.
        /// <para/>
        /// This can be combined with ExportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public abstract Task ImportStackAsync(string stackName, StackDeployment state, CancellationToken cancellationToken = default);

        /// <summary>
        /// Installs a plugin in the Workspace, for example to use cloud providers like AWS or GCP.
        /// </summary>
        /// <param name="name">The name of the plugin.</param>
        /// <param name="version">The version of the plugin e.g. "v1.0.0".</param>
        /// <param name="kind">The kind of plugin e.g. "resource".</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task InstallPluginAsync(string name, string version, PluginKind kind = PluginKind.Resource, CancellationToken cancellationToken = default);

        /// <summary>
        /// Removes a plugin from the Workspace matching the specified name and version.
        /// </summary>
        /// <param name="name">The optional name of the plugin.</param>
        /// <param name="versionRange">The optional semver range to check when removing plugins matching the given name e.g. "1.0.0", ">1.0.0".</param>
        /// <param name="kind">The kind of plugin e.g. "resource".</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task RemovePluginAsync(string? name = null, string? versionRange = null, PluginKind kind = PluginKind.Resource, CancellationToken cancellationToken = default);

        /// <summary>
        /// Returns a list of all plugins installed in the Workspace.
        /// </summary>
        public abstract Task<ImmutableList<PluginInfo>> ListPluginsAsync(CancellationToken cancellationToken = default);

        /// <summary>
        /// Gets the current set of Stack outputs from the last <see cref="WorkspaceStack.UpAsync(UpOptions?, CancellationToken)"/>.
        /// </summary>
        /// <param name="stackName">The name of the stack.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public abstract Task<ImmutableDictionary<string, OutputValue>> GetStackOutputsAsync(string stackName, CancellationToken cancellationToken = default);

        internal async Task<CommandResult> RunStackCommandAsync(
            string stackName,
            IList<string> args,
            Action<string>? onStandardOutput,
            Action<string>? onStandardError,
            Action<EngineEvent>? onEngineEvent,
            CancellationToken cancellationToken)
        {
            var additionalArgs = await this.SerializeArgsForOpAsync(stackName, cancellationToken).ConfigureAwait(false);
            var completeArgs = args.Concat(additionalArgs).ToList();

            var result = await this.RunCommandAsync(completeArgs, onStandardOutput, onStandardError, onEngineEvent, cancellationToken).ConfigureAwait(false);
            await this.PostCommandCallbackAsync(stackName, cancellationToken).ConfigureAwait(false);
            return result;
        }

        internal Task<CommandResult> RunCommandAsync(
            IList<string> args,
            CancellationToken cancellationToken)
            => this.RunCommandAsync(args, onStandardOutput: null, onStandardError: null, onEngineEvent: null, cancellationToken);

        internal Task<CommandResult> RunCommandAsync(
            IList<string> args,
            Action<string>? onStandardOutput,
            Action<string>? onStandardError,
            Action<EngineEvent>? onEngineEvent,
            CancellationToken cancellationToken)
        {
            var env = new Dictionary<string, string?>();
            if (!string.IsNullOrWhiteSpace(this.PulumiHome))
                env["PULUMI_HOME"] = this.PulumiHome;

            if (this.EnvironmentVariables != null)
            {
                foreach (var pair in this.EnvironmentVariables)
                    env[pair.Key] = pair.Value;
            }

            return this._cmd.RunAsync(args, this.WorkDir, env, onStandardOutput, onStandardError, onEngineEvent, cancellationToken);
        }

        public virtual void Dispose()
        {
        }
    }
}
