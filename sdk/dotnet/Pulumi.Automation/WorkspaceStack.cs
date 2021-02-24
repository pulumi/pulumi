// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Net;
using System.Runtime.ExceptionServices;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Pulumi.Automation.Commands;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Serialization;

namespace Pulumi.Automation
{
    /// <summary>
    /// <see cref="WorkspaceStack"/> is an isolated, independently configurable instance of a
    /// Pulumi program. <see cref="WorkspaceStack"/> exposes methods for the full pulumi lifecycle
    /// (up/preview/refresh/destroy), as well as managing configuration.
    /// <para/>
    /// Multiple stacks are commonly used to denote different phases of development
    /// (such as development, staging, and production) or feature branches (such as
    /// feature-x-dev, jane-feature-x-dev).
    /// <para/>
    /// Will dispose the <see cref="Workspace"/> on <see cref="Dispose"/>.
    /// </summary>
    public sealed class WorkspaceStack : IDisposable
    {
        private readonly Task _readyTask;

        /// <summary>
        /// The name identifying the Stack.
        /// </summary>
        public string Name { get; }

        /// <summary>
        /// The Workspace the Stack was created from.
        /// </summary>
        public Workspace Workspace { get; }

        /// <summary>
        /// Creates a new stack using the given workspace, and stack name.
        /// It fails if a stack with that name already exists.
        /// </summary>
        /// <param name="name">The name identifying the stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        /// <exception cref="StackAlreadyExistsException">If a stack with the provided name already exists.</exception>
        public static async Task<WorkspaceStack> CreateAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new WorkspaceStack(name, workspace, WorkspaceStackInitMode.Create, cancellationToken);
            await stack._readyTask.ConfigureAwait(false);
            return stack;
        }

        /// <summary>
        /// Selects stack using the given workspace, and stack name.
        /// It returns an error if the given Stack does not exist.
        /// </summary>
        /// <param name="name">The name identifying the stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        /// <exception cref="StackNotFoundException">If a stack with the provided name does not exists.</exception>
        public static async Task<WorkspaceStack> SelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new WorkspaceStack(name, workspace, WorkspaceStackInitMode.Select, cancellationToken);
            await stack._readyTask.ConfigureAwait(false);
            return stack;
        }

        /// <summary>
        /// Tries to create a new Stack using the given workspace, and stack name
        /// if the stack does not already exist, or falls back to selecting an
        /// existing stack. If the stack does not exist, it will be created and
        /// selected.
        /// </summary>
        /// <param name="name">The name of the identifying stack.</param>
        /// <param name="workspace">The Workspace the Stack was created from.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public static async Task<WorkspaceStack> CreateOrSelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new WorkspaceStack(name, workspace, WorkspaceStackInitMode.CreateOrSelect, cancellationToken);
            await stack._readyTask.ConfigureAwait(false);
            return stack;
        }

        private WorkspaceStack(
            string name,
            Workspace workspace,
            WorkspaceStackInitMode mode,
            CancellationToken cancellationToken)
        {
            this.Name = name;
            this.Workspace = workspace;

            switch (mode)
            {
                case WorkspaceStackInitMode.Create:
                    this._readyTask = workspace.CreateStackAsync(name, cancellationToken);
                    break;
                case WorkspaceStackInitMode.Select:
                    this._readyTask = workspace.SelectStackAsync(name, cancellationToken);
                    break;
                case WorkspaceStackInitMode.CreateOrSelect:
                    this._readyTask = Task.Run(async () =>
                    {
                        try
                        {
                            await workspace.CreateStackAsync(name, cancellationToken).ConfigureAwait(false);
                        }
                        catch (StackAlreadyExistsException)
                        {
                            await workspace.SelectStackAsync(name, cancellationToken).ConfigureAwait(false);
                        }
                    });
                    break;
                default:
                    throw new InvalidOperationException($"Unexpected Stack creation mode: {mode}");
            }
        }

        /// <summary>
        /// Returns the config value associated with the specified key.
        /// </summary>
        /// <param name="key">The key to use for the config lookup.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ConfigValue> GetConfigValueAsync(string key, CancellationToken cancellationToken = default)
            => this.Workspace.GetConfigValueAsync(this.Name, key, cancellationToken);

        /// <summary>
        /// Returns the full config map associated with the stack in the Workspace.
        /// </summary>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ImmutableDictionary<string, ConfigValue>> GetConfigAsync(CancellationToken cancellationToken = default)
            => this.Workspace.GetConfigAsync(this.Name, cancellationToken);

        /// <summary>
        /// Sets the config key-value pair on the Stack in the associated Workspace.
        /// </summary>
        /// <param name="key">The key to set.</param>
        /// <param name="value">The config value to set.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task SetConfigValueAsync(string key, ConfigValue value, CancellationToken cancellationToken = default)
            => this.Workspace.SetConfigValueAsync(this.Name, key, value, cancellationToken);

        /// <summary>
        /// Sets all specified config values on the stack in the associated Workspace.
        /// </summary>
        /// <param name="configMap">The map of config key-value pairs to set.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task SetConfigAsync(IDictionary<string, ConfigValue> configMap, CancellationToken cancellationToken = default)
            => this.Workspace.SetConfigAsync(this.Name, configMap, cancellationToken);

        /// <summary>
        /// Removes the specified config key from the Stack in the associated Workspace.
        /// </summary>
        /// <param name="key">The config key to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task RemoveConfigValueAsync(string key, CancellationToken cancellationToken = default)
            => this.Workspace.RemoveConfigValueAsync(this.Name, key, cancellationToken);

        /// <summary>
        /// Removes the specified config keys from the Stack in the associated Workspace.
        /// </summary>
        /// <param name="keys">The config keys to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task RemoveConfigAsync(IEnumerable<string> keys, CancellationToken cancellationToken = default)
            => this.Workspace.RemoveConfigAsync(this.Name, keys, cancellationToken);

        /// <summary>
        /// Gets and sets the config map used with the last update.
        /// </summary>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ImmutableDictionary<string, ConfigValue>> RefreshConfigAsync(CancellationToken cancellationToken = default)
            => this.Workspace.RefreshConfigAsync(this.Name, cancellationToken);

        /// <summary>
        /// Creates or updates the resources in a stack by executing the program in the Workspace.
        /// <para/>
        /// https://www.pulumi.com/docs/reference/cli/pulumi_up/
        /// </summary>
        /// <param name="options">Options to customize the behavior of the update.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<UpResult> UpAsync(
            UpOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            await this.Workspace.SelectStackAsync(this.Name, cancellationToken).ConfigureAwait(false);
            var execKind = ExecKind.Local;
            var program = this.Workspace.Program;
            var args = new List<string>()
            {
                "up",
                "--yes",
                "--skip-preview",
            };

            if (options != null)
            {
                if (options.Program != null)
                    program = options.Program;

                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.ExpectNoChanges is true)
                    args.Add("--expect-no-changes");

                if (options.Replace?.Any() == true)
                {
                    foreach (var item in options.Replace)
                    {
                        args.Add("--replace");
                        args.Add(item);
                    }
                }

                if (options.Target?.Any() == true)
                {
                    foreach (var item in options.Target)
                    {
                        args.Add("--target");
                        args.Add(item);
                    }
                }

                if (options.TargetDependents is true)
                    args.Add("--target-dependents");

                if (options.Parallel.HasValue)
                {
                    args.Add("--parallel");
                    args.Add(options.Parallel.Value.ToString());
                }
            }

            InlineLanguageHost? inlineHost = null;
            try
            {
                if (program != null)
                {
                    execKind = ExecKind.Inline;
                    inlineHost = new InlineLanguageHost(program, cancellationToken);
                    await inlineHost.StartAsync().ConfigureAwait(false);
                    var port = await inlineHost.GetPortAsync().ConfigureAwait(false);
                    args.Add($"--client=127.0.0.1:{port}");
                }

                args.Add("--exec-kind");
                args.Add(execKind);

                var upResult = await this.RunCommandAsync(args, options?.OnOutput, cancellationToken).ConfigureAwait(false);
                if (inlineHost != null && inlineHost.TryGetExceptionInfo(out var exceptionInfo))
                    exceptionInfo.Throw();

                var output = await this.GetOutputAsync(cancellationToken).ConfigureAwait(false);
                var summary = await this.GetInfoAsync(cancellationToken).ConfigureAwait(false);
                return new UpResult(
                    upResult.StandardOutput,
                    upResult.StandardError,
                    summary!,
                    output);
            }
            finally
            {
                if (inlineHost != null)
                {
                    await inlineHost.DisposeAsync().ConfigureAwait(false);
                }
            }
        }

        /// <summary>
        /// Performs a dry-run update to a stack, returning pending changes.
        /// <para/>
        /// https://www.pulumi.com/docs/reference/cli/pulumi_preview/
        /// </summary>
        /// <param name="options">Options to customize the behavior of the update.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<PreviewResult> PreviewAsync(
            PreviewOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            await this.Workspace.SelectStackAsync(this.Name, cancellationToken).ConfigureAwait(false);
            var execKind = ExecKind.Local;
            var program = this.Workspace.Program;
            var args = new List<string>() { "preview" };

            if (options != null)
            {
                if (options.Program != null)
                    program = options.Program;

                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.ExpectNoChanges is true)
                    args.Add("--expect-no-changes");

                if (options.Replace?.Any() == true)
                {
                    foreach (var item in options.Replace)
                    {
                        args.Add("--replace");
                        args.Add(item);
                    }
                }

                if (options.Target?.Any() == true)
                {
                    foreach (var item in options.Target)
                    {
                        args.Add("--target");
                        args.Add(item);
                    }
                }

                if (options.TargetDependents is true)
                    args.Add("--target-dependents");

                if (options.Parallel.HasValue)
                {
                    args.Add("--parallel");
                    args.Add(options.Parallel.Value.ToString());
                }
            }

            InlineLanguageHost? inlineHost = null;
            try
            {
                if (program != null)
                {
                    execKind = ExecKind.Inline;
                    inlineHost = new InlineLanguageHost(program, cancellationToken);
                    await inlineHost.StartAsync().ConfigureAwait(false);
                    var port = await inlineHost.GetPortAsync().ConfigureAwait(false);
                    args.Add($"--client=127.0.0.1:{port}");
                }

                args.Add("--exec-kind");
                args.Add(execKind);

                var upResult = await this.RunCommandAsync(args, null, cancellationToken).ConfigureAwait(false);
                if (inlineHost != null && inlineHost.TryGetExceptionInfo(out var exceptionInfo))
                    exceptionInfo.Throw();

                return new PreviewResult(
                    upResult.StandardOutput,
                    upResult.StandardError);
            }
            finally
            {
                if (inlineHost != null)
                {
                    await inlineHost.DisposeAsync().ConfigureAwait(false);
                }
            }
        }

        /// <summary>
        /// Compares the current stack’s resource state with the state known to exist in the actual
        /// cloud provider. Any such changes are adopted into the current stack.
        /// </summary>
        /// <param name="options">Options to customize the behavior of the refresh.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<UpdateResult> RefreshAsync(
            RefreshOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            await this.Workspace.SelectStackAsync(this.Name, cancellationToken).ConfigureAwait(false);
            var args = new List<string>()
            {
                "refresh",
                "--yes",
                "--skip-preview",
            };

            if (options != null)
            {
                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.ExpectNoChanges is true)
                    args.Add("--expect-no-changes");

                if (options.Target?.Any() == true)
                {
                    foreach (var item in options.Target)
                    {
                        args.Add("--target");
                        args.Add(item);
                    }
                }

                if (options.Parallel.HasValue)
                {
                    args.Add("--parallel");
                    args.Add(options.Parallel.Value.ToString());
                }
            }

            var result = await this.RunCommandAsync(args, options?.OnOutput, cancellationToken).ConfigureAwait(false);
            var summary = await this.GetInfoAsync(cancellationToken).ConfigureAwait(false);
            return new UpdateResult(
                result.StandardOutput,
                result.StandardError,
                summary!);
        }

        /// <summary>
        /// Destroy deletes all resources in a stack, leaving all history and configuration intact.
        /// </summary>
        /// <param name="options">Options to customize the behavior of the destroy.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<UpdateResult> DestroyAsync(
            DestroyOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            await this.Workspace.SelectStackAsync(this.Name, cancellationToken).ConfigureAwait(false);
            var args = new List<string>()
            {
                "destroy",
                "--yes",
                "--skip-preview",
            };

            if (options != null)
            {
                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.Target?.Any() == true)
                {
                    foreach (var item in options.Target)
                    {
                        args.Add("--target");
                        args.Add(item);
                    }
                }

                if (options.TargetDependents is true)
                    args.Add("--target-dependents");

                if (options.Parallel.HasValue)
                {
                    args.Add("--parallel");
                    args.Add(options.Parallel.Value.ToString());
                }
            }

            var result = await this.RunCommandAsync(args, options?.OnOutput, cancellationToken).ConfigureAwait(false);
            var summary = await this.GetInfoAsync(cancellationToken).ConfigureAwait(false);
            return new UpdateResult(
                result.StandardOutput,
                result.StandardError,
                summary!);
        }

        /// <summary>
        /// Gets the current set of Stack outputs from the last <see cref="UpAsync(UpOptions?, CancellationToken)"/>.
        /// </summary>
        private async Task<ImmutableDictionary<string, OutputValue>> GetOutputAsync(CancellationToken cancellationToken)
        {
            await this.Workspace.SelectStackAsync(this.Name).ConfigureAwait(false);

            // TODO: do this in parallel after this is fixed https://github.com/pulumi/pulumi/issues/6050
            var maskedResult = await this.RunCommandAsync(new[] { "stack", "output", "--json" }, null, cancellationToken).ConfigureAwait(false);
            var plaintextResult = await this.RunCommandAsync(new[] { "stack", "output", "--json", "--show-secrets" }, null, cancellationToken).ConfigureAwait(false);
            var jsonOptions = LocalSerializer.BuildJsonSerializerOptions();
            var maskedOutput = JsonSerializer.Deserialize<Dictionary<string, object>>(maskedResult.StandardOutput, jsonOptions);
            var plaintextOutput = JsonSerializer.Deserialize<Dictionary<string, object>>(plaintextResult.StandardOutput, jsonOptions);
            
            var output = new Dictionary<string, OutputValue>();
            foreach (var (key, value) in plaintextOutput)
            {
                var secret = maskedOutput[key] is string maskedValue && maskedValue == "[secret]";
                output[key] = new OutputValue(value, secret);
            }

            return output.ToImmutableDictionary();
        }

        /// <summary>
        /// Returns a list summarizing all previews and current results from Stack lifecycle operations (up/preview/refresh/destroy).
        /// </summary>
        /// <param name="options">Options to customize the behavior of the fetch history action.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<ImmutableList<UpdateSummary>> GetHistoryAsync(
            HistoryOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var args = new List<string>()
            {
                "history",
                "--json",
                "--show-secrets",
            };

            if (options?.PageSize.HasValue == true)
            {
                if (options.PageSize!.Value < 1)
                    throw new ArgumentException($"{nameof(options.PageSize)} must be greater than or equal to 1.", nameof(options.PageSize));

                var page = !options.Page.HasValue ? 1
                    : options.Page.Value < 1 ? 1
                    : options.Page.Value;

                args.Add("--page-size");
                args.Add(options.PageSize.Value.ToString());
                args.Add("--page");
                args.Add(page.ToString());
            }

            var result = await this.RunCommandAsync(args, null, cancellationToken).ConfigureAwait(false);
            var jsonOptions = LocalSerializer.BuildJsonSerializerOptions();
            var list = JsonSerializer.Deserialize<List<UpdateSummary>>(result.StandardOutput, jsonOptions);
            return list.ToImmutableList();
        }

        public async Task<UpdateSummary?> GetInfoAsync(CancellationToken cancellationToken = default)
        {
            var history = await this.GetHistoryAsync(
                new HistoryOptions
                {
                    PageSize = 1,
                },
                cancellationToken).ConfigureAwait(false);

            return history.FirstOrDefault();
        }

        private Task<CommandResult> RunCommandAsync(
            IEnumerable<string> args,
            Action<string>? onOutput,
            CancellationToken cancellationToken)
            => this.Workspace.RunStackCommandAsync(this.Name, args, onOutput, cancellationToken);

        public void Dispose()
            => this.Workspace.Dispose();

        private static class ExecKind
        {
            public const string Local = "auto.local";
            public const string Inline = "auto.inline";
        }

        private enum WorkspaceStackInitMode
        {
            Create,
            Select,
            CreateOrSelect
        }

        private class InlineLanguageHost : IAsyncDisposable
        {
            private readonly TaskCompletionSource<int> _portTcs = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
            private readonly CancellationToken _cancelToken;
            private readonly IHost _host;
            private readonly CancellationTokenRegistration _portRegistration;

            public InlineLanguageHost(
                PulumiFn program,
                CancellationToken cancellationToken)
            {
                this._cancelToken = cancellationToken;
                this._host = Host.CreateDefaultBuilder()
                    .ConfigureWebHostDefaults(webBuilder =>
                    {
                        webBuilder
                            .ConfigureKestrel(kestrelOptions =>
                            {
                                kestrelOptions.Listen(IPAddress.Any, 0, listenOptions =>
                                {
                                    listenOptions.Protocols = HttpProtocols.Http2;
                                });
                            })
                            .ConfigureServices(services =>
                            {
                                services.AddLogging();

                                // to be injected into LanguageRuntimeService
                                var callerContext = new LanguageRuntimeService.CallerContext(program, cancellationToken);
                                services.AddSingleton(callerContext);

                                services.AddGrpc(grpcOptions =>
                                {
                                    grpcOptions.MaxReceiveMessageSize = LanguageRuntimeService.MaxRpcMesageSize;
                                    grpcOptions.MaxSendMessageSize = LanguageRuntimeService.MaxRpcMesageSize;
                                });
                            })
                            .Configure(app =>
                            {
                                app.UseRouting();
                                app.UseEndpoints(endpoints =>
                                {
                                    endpoints.MapGrpcService<LanguageRuntimeService>();
                                });
                            });
                    })
                    .Build();

                // before starting the host, set up this callback to tell us what port was selected
                this._portRegistration = this._host.Services.GetRequiredService<IHostApplicationLifetime>().ApplicationStarted.Register(() =>
                {
                    try
                    {
                        var serverFeatures = this._host.Services.GetRequiredService<IServer>().Features;
                        var addresses = serverFeatures.Get<IServerAddressesFeature>().Addresses.ToList();
                        Debug.Assert(addresses.Count == 1, "Server should only be listening on one address");
                        var uri = new Uri(addresses[0]);
                        this._portTcs.TrySetResult(uri.Port);
                    }
                    catch (Exception ex)
                    {
                        this._portTcs.TrySetException(ex);
                    }
                });
            }

            public Task StartAsync()
                => this._host.StartAsync(this._cancelToken);

            public Task<int> GetPortAsync()
                => this._portTcs.Task;

            public bool TryGetExceptionInfo([NotNullWhen(true)] out ExceptionDispatchInfo? info)
            {
                var callerContext = this._host.Services.GetRequiredService<LanguageRuntimeService.CallerContext>();
                if (callerContext.ExceptionDispatchInfo is null)
                {
                    info = null;
                    return false;
                }

                info = callerContext.ExceptionDispatchInfo;
                return true;
            }

            public async ValueTask DisposeAsync()
            {
                this._portRegistration.Unregister();
                await this._host.StopAsync(this._cancelToken).ConfigureAwait(false);
                this._host.Dispose();
            }
        }
    }
}
