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
using Microsoft.Extensions.Logging;
using Pulumi.Automation.Commands;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;
using Pulumi.Automation.Exceptions;
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

            this._readyTask = mode switch
            {
                WorkspaceStackInitMode.Create => workspace.CreateStackAsync(name, cancellationToken),
                WorkspaceStackInitMode.Select => workspace.SelectStackAsync(name, cancellationToken),
                WorkspaceStackInitMode.CreateOrSelect => Task.Run(async () =>
                {
                    try
                    {
                        await workspace.CreateStackAsync(name, cancellationToken).ConfigureAwait(false);
                    }
                    catch (StackAlreadyExistsException)
                    {
                        await workspace.SelectStackAsync(name, cancellationToken).ConfigureAwait(false);
                    }
                }),
                _ => throw new InvalidOperationException($"Unexpected Stack creation mode: {mode}")
            };
        }

        /// <summary>
        /// Returns the config value associated with the specified key.
        /// </summary>
        /// <param name="key">The key to use for the config lookup.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ConfigValue> GetConfigAsync(string key, CancellationToken cancellationToken = default)
            => this.Workspace.GetConfigAsync(this.Name, key, cancellationToken);

        /// <summary>
        /// Returns the full config map associated with the stack in the Workspace.
        /// </summary>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task<ImmutableDictionary<string, ConfigValue>> GetAllConfigAsync(CancellationToken cancellationToken = default)
            => this.Workspace.GetAllConfigAsync(this.Name, cancellationToken);

        /// <summary>
        /// Sets the config key-value pair on the Stack in the associated Workspace.
        /// </summary>
        /// <param name="key">The key to set.</param>
        /// <param name="value">The config value to set.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task SetConfigAsync(string key, ConfigValue value, CancellationToken cancellationToken = default)
            => this.Workspace.SetConfigAsync(this.Name, key, value, cancellationToken);

        /// <summary>
        /// Sets all specified config values on the stack in the associated Workspace.
        /// </summary>
        /// <param name="configMap">The map of config key-value pairs to set.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task SetAllConfigAsync(IDictionary<string, ConfigValue> configMap, CancellationToken cancellationToken = default)
            => this.Workspace.SetAllConfigAsync(this.Name, configMap, cancellationToken);

        /// <summary>
        /// Removes the specified config key from the Stack in the associated Workspace.
        /// </summary>
        /// <param name="key">The config key to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task RemoveConfigAsync(string key, CancellationToken cancellationToken = default)
            => this.Workspace.RemoveConfigAsync(this.Name, key, cancellationToken);

        /// <summary>
        /// Removes the specified config keys from the Stack in the associated Workspace.
        /// </summary>
        /// <param name="keys">The config keys to remove.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public Task RemoveAllConfigAsync(IEnumerable<string> keys, CancellationToken cancellationToken = default)
            => this.Workspace.RemoveAllConfigAsync(this.Name, keys, cancellationToken);

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
            var execKind = ExecKind.Local;
            var program = this.Workspace.Program;
            var logger = this.Workspace.Logger;
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

                if (options.Logger != null)
                    logger = options.Logger;

                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.ExpectNoChanges is true)
                    args.Add("--expect-no-changes");

                if (options.Diff is true)
                    args.Add("--diff");

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
                    inlineHost = new InlineLanguageHost(program, logger, cancellationToken);
                    await inlineHost.StartAsync().ConfigureAwait(false);
                    var port = await inlineHost.GetPortAsync().ConfigureAwait(false);
                    args.Add($"--client=127.0.0.1:{port}");
                }

                args.Add("--exec-kind");
                args.Add(execKind);

                CommandResult upResult;
                try
                {
                    upResult = await this.RunCommandAsync(
                        args,
                        options?.OnStandardOutput,
                        options?.OnStandardError,
                        options?.OnEvent,
                        cancellationToken).ConfigureAwait(false);
                }
                catch
                {
                    if (inlineHost != null && inlineHost.TryGetExceptionInfo(out var exceptionInfo))
                        exceptionInfo.Throw();

                    // this won't be hit if we have an inline
                    // program exception
                    throw;
                }

                var output = await this.GetOutputsAsync(cancellationToken).ConfigureAwait(false);
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
            var execKind = ExecKind.Local;
            var program = this.Workspace.Program;
            var logger = this.Workspace.Logger;
            var args = new List<string>() { "preview" };

            if (options != null)
            {
                if (options.Program != null)
                    program = options.Program;

                if (options.Logger != null)
                    logger = options.Logger;

                if (!string.IsNullOrWhiteSpace(options.Message))
                {
                    args.Add("--message");
                    args.Add(options.Message);
                }

                if (options.ExpectNoChanges is true)
                    args.Add("--expect-no-changes");

                if (options.Diff is true)
                    args.Add("--diff");

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

            SummaryEvent? summaryEvent = null;

            var onEvent = options?.OnEvent;

            void OnPreviewEvent(EngineEvent @event)
            {
                if (@event.SummaryEvent != null)
                {
                    summaryEvent = @event.SummaryEvent;
                }

                onEvent?.Invoke(@event);
            }

            try
            {
                if (program != null)
                {
                    execKind = ExecKind.Inline;
                    inlineHost = new InlineLanguageHost(program, logger, cancellationToken);
                    await inlineHost.StartAsync().ConfigureAwait(false);
                    var port = await inlineHost.GetPortAsync().ConfigureAwait(false);
                    args.Add($"--client=127.0.0.1:{port}");
                }

                args.Add("--exec-kind");
                args.Add(execKind);

                CommandResult result;
                try
                {
                    result = await this.RunCommandAsync(
                        args,
                        options?.OnStandardOutput,
                        options?.OnStandardError,
                        OnPreviewEvent,
                        cancellationToken).ConfigureAwait(false);
                }
                catch
                {
                    if (inlineHost != null && inlineHost.TryGetExceptionInfo(out var exceptionInfo))
                        exceptionInfo.Throw();

                    // this won't be hit if we have an inline
                    // program exception
                    throw;
                }

                if (summaryEvent is null)
                {
                    throw new NoSummaryEventException("No summary of changes for 'preview'");
                }

                return new PreviewResult(
                    result.StandardOutput,
                    result.StandardError,
                    summaryEvent.ResourceChanges);
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
            var args = new List<string>
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

            var execKind = Workspace.Program is null ? ExecKind.Local : ExecKind.Inline;
            args.Add("--exec-kind");
            args.Add(execKind);

            var result = await this.RunCommandAsync(args, options?.OnStandardOutput, options?.OnStandardError, options?.OnEvent, cancellationToken).ConfigureAwait(false);
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
            var args = new List<string>
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

            var execKind = Workspace.Program is null ? ExecKind.Local : ExecKind.Inline;
            args.Add("--exec-kind");
            args.Add(execKind);

            var result = await this.RunCommandAsync(args, options?.OnStandardOutput, options?.OnStandardError, options?.OnEvent, cancellationToken).ConfigureAwait(false);
            var summary = await this.GetInfoAsync(cancellationToken).ConfigureAwait(false);
            return new UpdateResult(
                result.StandardOutput,
                result.StandardError,
                summary!);
        }

        /// <summary>
        /// Gets the current set of Stack outputs from the last <see cref="UpAsync(UpOptions?, CancellationToken)"/>.
        /// </summary>
        public Task<ImmutableDictionary<string, OutputValue>> GetOutputsAsync(CancellationToken cancellationToken = default)
            => this.Workspace.GetStackOutputsAsync(this.Name, cancellationToken);

        /// <summary>
        /// Returns a list summarizing all previews and current results from Stack lifecycle operations (up/preview/refresh/destroy).
        /// </summary>
        /// <param name="options">Options to customize the behavior of the fetch history action.</param>
        /// <param name="cancellationToken">A cancellation token.</param>
        public async Task<ImmutableList<UpdateSummary>> GetHistoryAsync(
            HistoryOptions? options = null,
            CancellationToken cancellationToken = default)
        {
            var args = new List<string>
            {
                "stack",
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

            var result = await this.RunCommandAsync(args, null, null, null, cancellationToken).ConfigureAwait(false);
            if (string.IsNullOrWhiteSpace(result.StandardOutput))
                return ImmutableList<UpdateSummary>.Empty;

            var jsonOptions = LocalSerializer.BuildJsonSerializerOptions();
            var list = JsonSerializer.Deserialize<List<UpdateSummary>>(result.StandardOutput, jsonOptions);
            return list.ToImmutableList();
        }

        /// <summary>
        /// Exports the deployment state of the stack.
        /// <para/>
        /// This can be combined with ImportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public Task<StackDeployment> ExportStackAsync(CancellationToken cancellationToken = default)
            => this.Workspace.ExportStackAsync(this.Name, cancellationToken);

        /// <summary>
        /// Imports the specified deployment state into a pre-existing stack.
        /// <para/>
        /// This can be combined with ExportStackAsync to edit a
        /// stack's state (such as recovery from failed deployments).
        /// </summary>
        public Task ImportStackAsync(StackDeployment state, CancellationToken cancellationToken = default)
            => this.Workspace.ImportStackAsync(this.Name, state, cancellationToken);

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

        /// <summary>
        /// Cancel stops a stack's currently running update. It throws
        /// an exception if no update is currently running. Note that
        /// this operation is _very dangerous_, and may leave the
        /// stack in an inconsistent state if a resource operation was
        /// pending when the update was canceled. This command is not
        /// supported for local backends.
        /// </summary>
        public async Task CancelAsync(CancellationToken cancellationToken = default)
        {
            await this.Workspace.RunCommandAsync(new[] { "cancel", "--stack", this.Name, "--yes" }, cancellationToken)
                .ConfigureAwait(false);
        }

        private async Task<CommandResult> RunCommandAsync(
            IList<string> args,
            Action<string>? onStandardOutput,
            Action<string>? onStandardError,
            Action<EngineEvent>? onEngineEvent,
            CancellationToken cancellationToken)
        {
            args = args.Concat(new[] { "--stack", this.Name }).ToList();
            return await this.Workspace.RunStackCommandAsync(this.Name, args, onStandardOutput, onStandardError, onEngineEvent, cancellationToken);
        }

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
                ILogger? logger,
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
                            .ConfigureAppConfiguration((context, config) =>
                            {
                                // clear so we don't read appsettings.json
                                // note that we also won't read environment variables for config
                                config.Sources.Clear();
                            })
                            .ConfigureLogging(loggingBuilder =>
                            {
                                // disable default logging
                                loggingBuilder.ClearProviders();
                            })
                            .ConfigureServices(services =>
                            {
                                // to be injected into LanguageRuntimeService
                                var callerContext = new LanguageRuntimeService.CallerContext(program, logger, cancellationToken);
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
