using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.X.Automation.Commands;

namespace Pulumi.X.Automation
{
    /// <summary>
    /// LocalWorkspace is a default implementation of the Workspace interface.
    /// <para/>
    /// A Workspace is the execution context containing a single Pulumi project, a program,
    /// and multiple stacks.Workspaces are used to manage the execution environment,
    /// providing various utilities such as plugin installation, environment configuration
    /// ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    /// <para/>
    /// LocalWorkspace relies on Pulumi.yaml and Pulumi.{stack}.yaml as the intermediate format
    /// for Project and Stack settings.Modifying ProjectSettings will
    /// alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.{stack}.yaml file.
    /// This is identical to the behavior of Pulumi CLI driven workspaces.
    /// </summary>
    public sealed class LocalWorkspace : Workspace, IDisposable
    {
        private readonly bool _ownsWorkingDir;
        private readonly Task _readyTask;

        /// <inheritdoc/>
        public override string WorkDir { get; }

        /// <inheritdoc/>
        public override string? PulumiHome { get; }

        /// <inheritdoc/>
        public override string? SecretsProvider { get; }

        /// <inheritdoc/>
        public override PulumiFn? Program { get; set; }

        /// <inheritdoc/>
        public override IDictionary<string, string>? EnvironmentVariables { get; set; }

        /// <summary>
        /// Creates a workspace using the specified options. Used for maximal control and
        /// customization of the underlying environment before any stacks are created or selected.
        /// </summary>
        /// <param name="options">Options used to configure the workspace.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static async Task<LocalWorkspace> CreateAsync(
            LocalWorkspaceOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var ws = new LocalWorkspace(options, cancellationToken);
            await ws._readyTask.ConfigureAwait(false);
            return ws;
        }

        /// <summary>
        /// Creates a Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> CreateStackAsync(InlineProgramArgs args)
            => CreateStackAsync(args, default);

        /// <summary>
        /// Creates a Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> CreateStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.CreateAsync, cancellationToken);

        /// <summary>
        /// Creates a Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> CreateStackAsync(LocalProgramArgs args)
            => CreateStackAsync(args, default);

        /// <summary>
        /// Creates a Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> CreateStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.CreateAsync, cancellationToken);

        /// <summary>
        /// Selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> SelectStackAsync(InlineProgramArgs args)
            => SelectStackAsync(args, default);

        /// <summary>
        /// Selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> SelectStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.SelectAsync, cancellationToken);

        /// <summary>
        /// Selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> SelectStackAsync(LocalProgramArgs args)
            => SelectStackAsync(args, default);

        /// <summary>
        /// Selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> SelectStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.SelectAsync, cancellationToken);

        /// <summary>
        /// Creates or selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> CreateOrSelectStackAsync(InlineProgramArgs args)
            => CreateOrSelectStackAsync(args, default);

        /// <summary>
        /// Creates or selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with and inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> CreateOrSelectStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.CreateOrSelectAsync, cancellationToken);

        /// <summary>
        /// Creates or selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<XStack> CreateOrSelectStackAsync(LocalProgramArgs args)
            => CreateOrSelectStackAsync(args, default);

        /// <summary>
        /// Creates or selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the local Pulumi CLI program
        /// from the specified <see cref="LocalWorkspaceOptions.WorkDir"/>. This is a way to create drivers
        /// on top of pre-existing Pulumi programs. This Workspace will pick up any available Settings
        /// files(Pulumi.yaml, Pulumi.{stack}.yaml).
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with a pre-configured Pulumi CLI program that
        ///     already exists on disk, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<XStack> CreateOrSelectStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, XStack.CreateOrSelectAsync, cancellationToken);

        private static async Task<XStack> CreateStackHelperAsync(
            InlineProgramArgs args,
            Func<string, Workspace, CancellationToken, Task<XStack>> initFunc,
            CancellationToken cancellationToken)
        {
            if (args.ProjectSettings == null)
                args.ProjectSettings = ProjectSettings.Default(args.ProjectName);

            var ws = new LocalWorkspace(args, cancellationToken);
            await ws._readyTask;

            return await initFunc(args.StackName, ws, cancellationToken);
        }

        private static async Task<XStack> CreateStackHelperAsync(
            LocalProgramArgs args,
            Func<string, Workspace, CancellationToken, Task<XStack>> initFunc,
            CancellationToken cancellationToken)
        {
            var ws = new LocalWorkspace(args, cancellationToken);
            await ws._readyTask;

            return await initFunc(args.StackName, ws, cancellationToken);
        }

        private LocalWorkspace(
            LocalWorkspaceOptions? options,
            CancellationToken cancellationToken)
        {
            string? dir = null;
            var readyTasks = new List<Task>();

            if (options != null)
            {
                if (!string.IsNullOrWhiteSpace(options.WorkDir))
                    dir = options.WorkDir;

                this.PulumiHome = options.PulumiHome;
                this.Program = options.Program;
                this.SecretsProvider = options.SecretsProvider;

                if (options.EnvironmentVariables != null)
                    this.EnvironmentVariables = new Dictionary<string, string>(options.EnvironmentVariables);

                if (options.ProjectSettings != null)
                    readyTasks.Add(this.SaveProjectSettingsAsync(options.ProjectSettings, cancellationToken));

                if (options.StackSettings != null && options.StackSettings.Any())
                {
                    foreach (var pair in options.StackSettings)
                        readyTasks.Add(this.SaveStackSettingsAsync(pair.Key, pair.Value, cancellationToken));
                }
            }

            if (string.IsNullOrWhiteSpace(dir))
            {
                // note that csharp doesn't guarantee that Path.GetRandomFileName returns a name
                // for a file or folder that doesn't already exist.
                // we should be OK with the "automation-" prefix but a collision is still
                // theoretically possible
                dir = Path.Combine(Path.GetTempPath(), $"automation-{Path.GetRandomFileName()}");
                Directory.CreateDirectory(dir);
                this._ownsWorkingDir = true;
            }

            this.WorkDir = dir;
            this._readyTask = Task.WhenAll(readyTasks);
        }

        /// <inheritdoc/>
        public override Task<ProjectSettings?> GetProjectSettingsAsync(CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task SaveProjectSettingsAsync(ProjectSettings settings, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<StackSettings?> GetStackSettingsAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task SaveStackSettingsAsync(string stackName, StackSettings settings, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<ImmutableList<string>> SerializeArgsForOpAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotSupportedException($"{nameof(LocalWorkspace)} does not support {nameof(SerializeArgsForOpAsync)}.");
        }

        /// <inheritdoc/>
        public override Task PostCommandCallbackAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotSupportedException($"{nameof(LocalWorkspace)} does not support {nameof(PostCommandCallbackAsync)}.");
        }

        /// <inheritdoc/>
        public override Task<ConfigValue> GetConfigValueAsync(string stackName, string key, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override async Task<ImmutableDictionary<string, ConfigValue>> GetConfigAsync(string stackName, CancellationToken cancellationToken = default)
        {
            await this.SelectStackAsync(stackName, cancellationToken);
            var result = await this.RunCommandAsync(new[] { "config", "--show-secrets", "--json" }, cancellationToken);
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task SetConfigValueAsync(string stackName, string key, ConfigValue value, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task SetConfigAsync(string stackName, IDictionary<string, ConfigValue> configMap, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task RemoveConfigAsync(string stackName, string key, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task RemoveAllConfigAsync(string stackName, IEnumerable<string> keys, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<WhoAmIResult> WhoAmIAsync(CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<StackInfo?> GetStackAsync(CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task CreateStackAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task SelectStackAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task RemoveStackAsync(string stackName, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<ImmutableList<StackInfo>> ListStacksAsync(CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task InstallPluginAsync(string name, string version, string? kind = null, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task RemovePluginAsync(string? name = null, string? versionRange = null, string? kind = null, CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        /// <inheritdoc/>
        public override Task<ImmutableList<PluginInfo>> ListPluginsAsync(CancellationToken cancellationToken = default)
        {
            throw new NotImplementedException();
        }

        private Task<CommandResult> RunCommandAsync(
            IEnumerable<string> args,
            CancellationToken cancellationToken)
        {
            var env = new Dictionary<string, string>();
            if (!string.IsNullOrWhiteSpace(this.PulumiHome))
                env["PULUMI_HOME"] = this.PulumiHome;

            if (this.EnvironmentVariables != null)
            {
                foreach (var pair in this.EnvironmentVariables)
                    env[pair.Key] = pair.Value;
            }

            return PulumiCmd.RunAsync(args, this.WorkDir, env, cancellationToken: cancellationToken);
        }

        public void Dispose()
        {
            if (this._ownsWorkingDir
                && !string.IsNullOrWhiteSpace(this.WorkDir)
                && Directory.Exists(this.WorkDir))
            {
                try
                {
                    Directory.Delete(this.WorkDir, true);
                }
                catch
                {
                    // allow graceful exit if for some reason
                    // we're not able to delete the directory
                    // will rely on OS to clean temp directory
                    // in this case.
                }
            }
        }
    }
}
