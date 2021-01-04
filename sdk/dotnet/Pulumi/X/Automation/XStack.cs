using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Linq;
using System.Net;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Pulumi.X.Automation.Commands;
using Pulumi.X.Automation.Commands.Exceptions;

namespace Pulumi.X.Automation
{
    public sealed class XStack : IDisposable // TODO: come up with a name for this
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
        public static async Task<XStack> CreateAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.Create, cancellationToken);
            await stack._readyTask;
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
        public static async Task<XStack> SelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.Select, cancellationToken);
            await stack._readyTask;
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
        public static async Task<XStack> CreateOrSelectAsync(
            string name,
            Workspace workspace,
            CancellationToken cancellationToken = default)
        {
            var stack = new XStack(name, workspace, StackInitMode.CreateOrSelect, cancellationToken);
            await stack._readyTask;
            return stack;
        }

        private XStack(
            string name,
            Workspace workspace,
            StackInitMode mode,
            CancellationToken cancellationToken)
        {
            this.Name = name;
            this.Workspace = workspace;

            switch (mode)
            {
                case StackInitMode.Create:
                    this._readyTask = workspace.CreateStackAsync(name, cancellationToken);
                    break;
                case StackInitMode.Select:
                    this._readyTask = workspace.SelectStackAsync(name, cancellationToken);
                    break;
                case StackInitMode.CreateOrSelect:
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
        /// Removes the specified config keys from the Stack in the assocaited Workspace.
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

        // TODO: docs, input UpOptions, return value
        public async Task Up(CancellationToken cancellationToken = default)
        {
            IHost? host = null;
            try
            {
                // just mimicking typescript implementation's behavior - TODO: extract this to a reusable helper
                if (this.Workspace.Program != null)
                {
                    var portTcs = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
                    host = Host.CreateDefaultBuilder()
                        .ConfigureWebHostDefaults(webBuilder =>
                        {
                            webBuilder
                                .ConfigureKestrel(kestrelOptions =>
                                {
                                    kestrelOptions.Listen(IPAddress.Any, 0, listenOptions =>
                                    {
                                        // TODO: not sure if we need to do anything special to mimic typescript implementation's grpc.ServerCredentials.createInsecure()
                                        listenOptions.UseHttps();
                                    });
                                })
                                .ConfigureServices(services =>
                                {
                                    services.AddSingleton(this.Workspace.Program); // to be injected into LanguageRuntimeService
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
                    host.Services.GetRequiredService<IHostApplicationLifetime>().ApplicationStarted.Register(() =>
                    {
                        try
                        {
                            var serverFeatures = host.Services.GetRequiredService<IServer>().Features;
                            var addresses = serverFeatures.Get<IServerAddressesFeature>().Addresses.ToList();
                            Debug.Assert(addresses.Count == 1, "Server should only be listening on one address");
                            var uri = new Uri(addresses[0]);
                            portTcs.TrySetResult(uri.Port);
                        }
                        catch (Exception ex)
                        {
                            portTcs.TrySetException(ex);
                        }
                    });
                    await host.StartAsync(cancellationToken);
                    var port = await portTcs.Task;
                    // TODO: args.Add($"--client=127.0.0.1:{port}"); and kind=inline etc
                }
            }
            finally
            {
                if (host != null)
                {
                    await host.StopAsync(cancellationToken);
                    host.Dispose();
                }
            }
        }

        private Task<CommandResult> RunCommandAsync(
            IEnumerable<string> args,
            Action<string>? onOutput,
            CancellationToken cancellationToken)
            => this.Workspace.RunStackCommandAsync(this.Name, args, onOutput, cancellationToken);

        public void Dispose()
            => this.Workspace.Dispose();

        private enum StackInitMode // TODO: change name of this as per XStack
        {
            Create,
            Select,
            CreateOrSelect
        }
    }
}
