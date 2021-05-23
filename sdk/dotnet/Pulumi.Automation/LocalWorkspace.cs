// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Pulumi.Automation.Commands;
using Pulumi.Automation.Exceptions;
using Pulumi.Automation.Serialization;
using Semver;

namespace Pulumi.Automation
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
    /// <para/>
    /// If not provided a working directory - causing LocalWorkspace to create a temp directory,
    /// than the temp directory will be cleaned up on <see cref="Dispose"/>.
    /// </summary>
    public sealed class LocalWorkspace : Workspace
    {
        private readonly LocalSerializer _serializer = new LocalSerializer();
        private readonly bool _ownsWorkingDir;
        private readonly Task _readyTask;
        private static readonly SemVersion _minimumVersion = SemVersion.Parse("3.1.0");

        /// <inheritdoc/>
        public override string WorkDir { get; }

        /// <inheritdoc/>
        public override string? PulumiHome { get; }

        private SemVersion? _pulumiVersion;
        /// <inheritdoc/>
        public override string PulumiVersion => _pulumiVersion?.ToString() ?? throw new InvalidOperationException("Failed to get Pulumi version.");

        /// <inheritdoc/>
        public override string? SecretsProvider { get; }

        /// <inheritdoc/>
        public override PulumiFn? Program { get; set; }

        /// <inheritdoc/>
        public override ILogger? Logger { get; set; }

        /// <inheritdoc/>
        public override IDictionary<string, string?>? EnvironmentVariables { get; set; }

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
            var ws = new LocalWorkspace(
                new LocalPulumiCmd(),
                options,
                cancellationToken);
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
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<WorkspaceStack> CreateStackAsync(InlineProgramArgs args)
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
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<WorkspaceStack> CreateStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateAsync, cancellationToken);

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
        public static Task<WorkspaceStack> CreateStackAsync(LocalProgramArgs args)
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
        public static Task<WorkspaceStack> CreateStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateAsync, cancellationToken);

        /// <summary>
        /// Selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<WorkspaceStack> SelectStackAsync(InlineProgramArgs args)
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
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<WorkspaceStack> SelectStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.SelectAsync, cancellationToken);

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
        public static Task<WorkspaceStack> SelectStackAsync(LocalProgramArgs args)
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
        public static Task<WorkspaceStack> SelectStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.SelectAsync, cancellationToken);

        /// <summary>
        /// Creates or selects an existing Stack with a <see cref="LocalWorkspace"/> utilizing the specified
        /// inline (in process) <see cref="LocalWorkspaceOptions.Program"/>. This program
        /// is fully debuggable and runs in process. If no <see cref="LocalWorkspaceOptions.ProjectSettings"/>
        /// option is specified, default project settings will be created on behalf of the user. Similarly, unless a
        /// <see cref="LocalWorkspaceOptions.WorkDir"/> option is specified, the working directory will default
        /// to a new temporary directory provided by the OS.
        /// </summary>
        /// <param name="args">
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        public static Task<WorkspaceStack> CreateOrSelectStackAsync(InlineProgramArgs args)
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
        ///     A set of arguments to initialize a Stack with an inline <see cref="PulumiFn"/> program
        ///     that runs in process, as well as any additional customizations to be applied to the
        ///     workspace.
        /// </param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static Task<WorkspaceStack> CreateOrSelectStackAsync(InlineProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateOrSelectAsync, cancellationToken);

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
        public static Task<WorkspaceStack> CreateOrSelectStackAsync(LocalProgramArgs args)
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
        public static Task<WorkspaceStack> CreateOrSelectStackAsync(LocalProgramArgs args, CancellationToken cancellationToken)
            => CreateStackHelperAsync(args, WorkspaceStack.CreateOrSelectAsync, cancellationToken);

        private static async Task<WorkspaceStack> CreateStackHelperAsync(
            InlineProgramArgs args,
            Func<string, Workspace, CancellationToken, Task<WorkspaceStack>> initFunc,
            CancellationToken cancellationToken)
        {
            if (args.ProjectSettings is null)
                throw new ArgumentNullException(nameof(args.ProjectSettings));

            var ws = new LocalWorkspace(
                new LocalPulumiCmd(),
                args,
                cancellationToken);
            await ws._readyTask.ConfigureAwait(false);

            return await initFunc(args.StackName, ws, cancellationToken).ConfigureAwait(false);
        }

        private static async Task<WorkspaceStack> CreateStackHelperAsync(
            LocalProgramArgs args,
            Func<string, Workspace, CancellationToken, Task<WorkspaceStack>> initFunc,
            CancellationToken cancellationToken)
        {
            var ws = new LocalWorkspace(
                new LocalPulumiCmd(),
                args,
                cancellationToken);
            await ws._readyTask.ConfigureAwait(false);

            return await initFunc(args.StackName, ws, cancellationToken).ConfigureAwait(false);
        }

        internal LocalWorkspace(
            IPulumiCmd cmd,
            LocalWorkspaceOptions? options,
            CancellationToken cancellationToken)
            : base(cmd)
        {
            string? dir = null;
            var readyTasks = new List<Task>();

            if (options != null)
            {
                if (!string.IsNullOrWhiteSpace(options.WorkDir))
                    dir = options.WorkDir;

                this.PulumiHome = options.PulumiHome;
                this.Program = options.Program;
                this.Logger = options.Logger;
                this.SecretsProvider = options.SecretsProvider;

                if (options.EnvironmentVariables != null)
                    this.EnvironmentVariables = new Dictionary<string, string?>(options.EnvironmentVariables);
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

            readyTasks.Add(this.PopulatePulumiVersionAsync(cancellationToken));

            if (options?.ProjectSettings != null)
            {
                readyTasks.Add(this.InitializeProjectSettingsAsync(options.ProjectSettings, cancellationToken));
            }

            if (options?.StackSettings != null && options.StackSettings.Any())
            {
                foreach (var pair in options.StackSettings)
                    readyTasks.Add(this.SaveStackSettingsAsync(pair.Key, pair.Value, cancellationToken));
            }

            this._readyTask = Task.WhenAll(readyTasks);
        }

        private async Task InitializeProjectSettingsAsync(ProjectSettings projectSettings,
                                                          CancellationToken cancellationToken)
        {
            // If given project settings, we want to write them out to
            // the working dir. We do not want to override existing
            // settings with default settings though.

            var existingSettings = await this.GetProjectSettingsAsync(cancellationToken);
            if (existingSettings == null)
            {
                await this.SaveProjectSettingsAsync(projectSettings, cancellationToken);
            }
            else if (!projectSettings.IsDefault &&
                     !ProjectSettings.Comparer.Equals(projectSettings, existingSettings))
            {
                var path = this.FindSettingsFile();
                throw new ProjectSettingsConflictException(path);
            }
        }

        private static readonly string[] _settingsExtensions = { ".yaml", ".yml", ".json" };

        private async Task PopulatePulumiVersionAsync(CancellationToken cancellationToken)
        {
            var result = await this.RunCommandAsync(new[] { "version" }, cancellationToken).ConfigureAwait(false);
            var versionString = result.StandardOutput.Trim();
            versionString = versionString.TrimStart('v');
            if (!SemVersion.TryParse(versionString, out var version))
            {
                throw new InvalidOperationException("Failed to get Pulumi version.");
            }
            var skipVersionCheckVar = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK";
            var hasSkipEnvVar = this.EnvironmentVariables?.ContainsKey(skipVersionCheckVar) ?? false;
            var optOut = hasSkipEnvVar || Environment.GetEnvironmentVariable(skipVersionCheckVar) != null;
            ValidatePulumiVersion(_minimumVersion, version, optOut);
            this._pulumiVersion = version;
        }

        internal static void ValidatePulumiVersion(SemVersion minVersion, SemVersion currentVersion, bool optOut)
        {
            if (optOut)
            {
                return;
            }
            if (minVersion.Major < currentVersion.Major)
            {
                throw new InvalidOperationException($"Major version mismatch. You are using Pulumi CLI version {currentVersion} with Automation SDK v{minVersion.Major}. Please update the SDK.");
            }
            if (minVersion > currentVersion)
            {
                throw new InvalidOperationException($"Minimum version requirement failed. The minimum CLI version requirement is {minVersion}, your current CLI version is {currentVersion}. Please update the Pulumi CLI.");
            }
        }

        /// <inheritdoc/>
        public override async Task<ProjectSettings?> GetProjectSettingsAsync(CancellationToken cancellationToken = default)
        {
            var path = this.FindSettingsFile();
            var isJson = Path.GetExtension(path) == ".json";
            if (!File.Exists(path))
            {
                return null;
            }
            var content = await File.ReadAllTextAsync(path, cancellationToken).ConfigureAwait(false);
            if (isJson)
            {
                return this._serializer.DeserializeJson<ProjectSettings>(content);
            }
            var model = this._serializer.DeserializeYaml<ProjectSettingsModel>(content);
            return model.Convert();
        }

        /// <inheritdoc/>
        public override Task SaveProjectSettingsAsync(ProjectSettings settings, CancellationToken cancellationToken = default)
        {
            var path = this.FindSettingsFile();
            var ext = Path.GetExtension(path);
            var content = ext == ".json" ? this._serializer.SerializeJson(settings) : this._serializer.SerializeYaml(settings);
            return File.WriteAllTextAsync(path, content, cancellationToken);
        }

        private string FindSettingsFile()
        {
            foreach (var ext in _settingsExtensions)
            {
                var testPath = Path.Combine(this.WorkDir, $"Pulumi{ext}");
                if (File.Exists(testPath))
                {
                    return testPath;
                }
            }
            var defaultPath = Path.Combine(this.WorkDir, "Pulumi.yaml");
            return defaultPath;
        }

        private static string GetStackSettingsName(string stackName)
        {
            var parts = stackName.Split('/');
            if (parts.Length < 1)
                return stackName;

            return parts[^1];
        }

        /// <inheritdoc/>
        public override async Task<StackSettings?> GetStackSettingsAsync(string stackName, CancellationToken cancellationToken = default)
        {
            var settingsName = GetStackSettingsName(stackName);

            foreach (var ext in _settingsExtensions)
            {
                var isJson = ext == ".json";
                var path = Path.Combine(this.WorkDir, $"Pulumi.{settingsName}{ext}");
                if (!File.Exists(path))
                    continue;

                var content = await File.ReadAllTextAsync(path, cancellationToken).ConfigureAwait(false);
                return isJson ? this._serializer.DeserializeJson<StackSettings>(content) : this._serializer.DeserializeYaml<StackSettings>(content);
            }

            return null;
        }

        /// <inheritdoc/>
        public override Task SaveStackSettingsAsync(string stackName, StackSettings settings, CancellationToken cancellationToken = default)
        {
            var settingsName = GetStackSettingsName(stackName);

            var foundExt = ".yaml";
            foreach (var ext in _settingsExtensions)
            {
                var testPath = Path.Combine(this.WorkDir, $"Pulumi.{settingsName}{ext}");
                if (File.Exists(testPath))
                {
                    foundExt = ext;
                    break;
                }
            }

            var path = Path.Combine(this.WorkDir, $"Pulumi.{settingsName}{foundExt}");
            var content = foundExt == ".json" ? this._serializer.SerializeJson(settings) : this._serializer.SerializeYaml(settings);
            return File.WriteAllTextAsync(path, content, cancellationToken);
        }

        /// <inheritdoc/>
        public override Task<ImmutableList<string>> SerializeArgsForOpAsync(string stackName, CancellationToken cancellationToken = default)
            => Task.FromResult(ImmutableList<string>.Empty);

        /// <inheritdoc/>
        public override Task PostCommandCallbackAsync(string stackName, CancellationToken cancellationToken = default)
            => Task.CompletedTask;

        /// <inheritdoc/>
        public override async Task<ConfigValue> GetConfigAsync(string stackName, string key, CancellationToken cancellationToken = default)
        {
            var result = await this.RunCommandAsync(new[] { "config", "get", key, "--json", "--stack", stackName }, cancellationToken).ConfigureAwait(false);
            return this._serializer.DeserializeJson<ConfigValue>(result.StandardOutput);
        }

        /// <inheritdoc/>
        public override async Task<ImmutableDictionary<string, ConfigValue>> GetAllConfigAsync(string stackName, CancellationToken cancellationToken = default)
        {
            var result = await this.RunCommandAsync(new[] { "config", "--show-secrets", "--json", "--stack", stackName }, cancellationToken).ConfigureAwait(false);
            if (string.IsNullOrWhiteSpace(result.StandardOutput))
                return ImmutableDictionary<string, ConfigValue>.Empty;

            var dict = this._serializer.DeserializeJson<Dictionary<string, ConfigValue>>(result.StandardOutput);
            return dict.ToImmutableDictionary();
        }

        /// <inheritdoc/>
        public override async Task SetConfigAsync(string stackName, string key, ConfigValue value, CancellationToken cancellationToken = default)
        {
            var secretArg = value.IsSecret ? "--secret" : "--plaintext";
            await this.RunCommandAsync(new[] { "config", "set", key, value.Value, secretArg, "--stack", stackName }, cancellationToken).ConfigureAwait(false);
        }

        /// <inheritdoc/>
        public override async Task SetAllConfigAsync(string stackName, IDictionary<string, ConfigValue> configMap, CancellationToken cancellationToken = default)
        {
            var args = new List<string> { "config", "set-all", "--stack", stackName };
            foreach (var (key, value) in configMap)
            {
                var secretArg = value.IsSecret ? "--secret" : "--plaintext";
                args.Add(secretArg);
                args.Add($"{key}={value.Value}");
            }
            await this.RunCommandAsync(args, cancellationToken).ConfigureAwait(false);
        }

        /// <inheritdoc/>
        public override async Task RemoveConfigAsync(string stackName, string key, CancellationToken cancellationToken = default)
        {
            await this.RunCommandAsync(new[] { "config", "rm", key, "--stack", stackName }, cancellationToken).ConfigureAwait(false);
        }

        /// <inheritdoc/>
        public override async Task RemoveAllConfigAsync(string stackName, IEnumerable<string> keys, CancellationToken cancellationToken = default)
        {
            var args = new List<string> { "config", "rm-all", "--stack", stackName };
            args.AddRange(keys);
            await this.RunCommandAsync(args, cancellationToken).ConfigureAwait(false);
        }

        /// <inheritdoc/>
        public override async Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(string stackName, CancellationToken cancellationToken = default)
        {
            await this.RunCommandAsync(new[] { "config", "refresh", "--force", "--stack", stackName }, cancellationToken).ConfigureAwait(false);
            return await this.GetAllConfigAsync(stackName, cancellationToken).ConfigureAwait(false);
        }

        /// <inheritdoc/>
        public override async Task<WhoAmIResult> WhoAmIAsync(CancellationToken cancellationToken = default)
        {
            var result = await this.RunCommandAsync(new[] { "whoami" }, cancellationToken).ConfigureAwait(false);
            return new WhoAmIResult(result.StandardOutput.Trim());
        }

        /// <inheritdoc/>
        public override Task CreateStackAsync(string stackName, CancellationToken cancellationToken)
        {
            var args = new List<string>
            {
                "stack",
                "init",
                stackName,
            };

            if (!string.IsNullOrWhiteSpace(this.SecretsProvider))
                args.AddRange(new[] { "--secrets-provider", this.SecretsProvider });

            return this.RunCommandAsync(args, cancellationToken);
        }

        /// <inheritdoc/>
        public override Task SelectStackAsync(string stackName, CancellationToken cancellationToken)
            => this.RunCommandAsync(new[] { "stack", "select", stackName }, cancellationToken);

        /// <inheritdoc/>
        public override Task RemoveStackAsync(string stackName, CancellationToken cancellationToken = default)
            => this.RunCommandAsync(new[] { "stack", "rm", "--yes", stackName }, cancellationToken);

        /// <inheritdoc/>
        public override async Task<ImmutableList<StackSummary>> ListStacksAsync(CancellationToken cancellationToken = default)
        {
            var result = await this.RunCommandAsync(new[] { "stack", "ls", "--json" }, cancellationToken).ConfigureAwait(false);
            if (string.IsNullOrWhiteSpace(result.StandardOutput))
                return ImmutableList<StackSummary>.Empty;

            var stacks = this._serializer.DeserializeJson<List<StackSummary>>(result.StandardOutput);
            return stacks.ToImmutableList();
        }

        /// <inheritdoc/>
        public override async Task<StackDeployment> ExportStackAsync(string stackName, CancellationToken cancellationToken = default)
        {
            var commandResult = await this.RunCommandAsync(
                new[] { "stack", "export", "--stack", stackName, "--show-secrets" },
                cancellationToken).ConfigureAwait(false);
            return StackDeployment.FromJsonString(commandResult.StandardOutput);
        }

        /// <inheritdoc/>
        public override async Task ImportStackAsync(string stackName, StackDeployment state, CancellationToken cancellationToken = default)
        {
            var tempFileName = Path.GetTempFileName();
            try
            {
                await File.WriteAllTextAsync(tempFileName, state.Json.GetRawText(), cancellationToken);
                await this.RunCommandAsync(new[] { "stack", "import", "--file", tempFileName, "--stack", stackName },
                                           cancellationToken).ConfigureAwait(false);
            }
            finally
            {
                File.Delete(tempFileName);
            }
        }

        /// <inheritdoc/>
        public override Task InstallPluginAsync(string name, string version, PluginKind kind = PluginKind.Resource, CancellationToken cancellationToken = default)
            => this.RunCommandAsync(new[] { "plugin", "install", kind.ToString().ToLower(), name, version }, cancellationToken);

        /// <inheritdoc/>
        public override Task RemovePluginAsync(string? name = null, string? versionRange = null, PluginKind kind = PluginKind.Resource, CancellationToken cancellationToken = default)
        {
            var args = new List<string>
            {
                "plugin",
                "rm",
                kind.ToString().ToLower(),
            };

            if (!string.IsNullOrWhiteSpace(name))
                args.Add(name);

            if (!string.IsNullOrWhiteSpace(versionRange))
                args.Add(versionRange);

            args.Add("--yes");
            return this.RunCommandAsync(args, cancellationToken);
        }

        /// <inheritdoc/>
        public override async Task<ImmutableList<PluginInfo>> ListPluginsAsync(CancellationToken cancellationToken = default)
        {
            var result = await this.RunCommandAsync(new[] { "plugin", "ls", "--json" }, cancellationToken).ConfigureAwait(false);
            var plugins = this._serializer.DeserializeJson<List<PluginInfo>>(result.StandardOutput);
            return plugins.ToImmutableList();
        }

        /// <inheritdoc/>
        public override async Task<ImmutableDictionary<string, OutputValue>> GetStackOutputsAsync(string stackName, CancellationToken cancellationToken = default)
        {
            // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
            var maskedResult = await this.RunCommandAsync(new[] { "stack", "output", "--json", "--stack", stackName }, cancellationToken).ConfigureAwait(false);
            var plaintextResult = await this.RunCommandAsync(new[] { "stack", "output", "--json", "--show-secrets", "--stack", stackName }, cancellationToken).ConfigureAwait(false);

            var maskedOutput = string.IsNullOrWhiteSpace(maskedResult.StandardOutput)
                ? new Dictionary<string, object>()
                : _serializer.DeserializeJson<Dictionary<string, object>>(maskedResult.StandardOutput);

            var plaintextOutput = string.IsNullOrWhiteSpace(plaintextResult.StandardOutput)
                ? new Dictionary<string, object>()
                : _serializer.DeserializeJson<Dictionary<string, object>>(plaintextResult.StandardOutput);

            var output = new Dictionary<string, OutputValue>();
            foreach (var (key, value) in plaintextOutput)
            {
                var secret = maskedOutput[key] is string maskedValue && maskedValue == "[secret]";
                output[key] = new OutputValue(value, secret);
            }

            return output.ToImmutableDictionary();
        }

        public override void Dispose()
        {
            base.Dispose();

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
