using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Grpc.Core;
using Microsoft.Extensions.Logging;
using Pulumi.X.Automation.Runtime;
using Pulumirpc;

namespace Pulumi.X.Automation
{
    internal class LanguageRuntimeService : LanguageRuntime.LanguageRuntimeBase, IDisposable
    {
        // MaxRpcMesageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        public const int MaxRpcMesageSize = 1024 * 1024 * 400;

        private readonly CancellationTokenSource _cts = new CancellationTokenSource();
        private CancellationToken Token => _cts.Token;
        private readonly SemaphoreSlim _semaphore = new SemaphoreSlim(1, 1);

        private readonly PulumiFn _program;
        private readonly ILogger<LanguageRuntimeService> _logger;
        private bool _isDisposed;

        public LanguageRuntimeService(
            PulumiFn program,
            ILogger<LanguageRuntimeService> logger)
        {
            this._program = program;
            this._logger = logger;
        }

        public override Task<Pulumirpc.PluginInfo> GetPluginInfo(Empty request, ServerCallContext context)
        {
            var pluginInfo = new Pulumirpc.PluginInfo
            {
                Version = "1.0.0"
            };

            return Task.FromResult(pluginInfo);
        }

        public override Task<GetRequiredPluginsResponse> GetRequiredPlugins(GetRequiredPluginsRequest request, ServerCallContext context)
        {
            var response = new GetRequiredPluginsResponse();
            return Task.FromResult(response);
        }

        public override async Task<RunResponse> Run(RunRequest request, ServerCallContext context)
        {
            var contextId = Guid.NewGuid();
            this._logger.LogInformation("Waiting for lock with id {0}", contextId);
            await this._semaphore.WaitAsync();

            this._logger.LogInformation("Obtained lock with id {0}", contextId);
            if (Token.IsCancellationRequested)
            {
                this._logger.LogInformation("Releasing lock with id {0}", contextId);
                this._semaphore.Release();
                return new RunResponse();
            }

            try
            {
                var args = request.Args;
                var engineAddr = args != null && args.Any() ? args[0] : "";

                var runInfo = new RuntimeSettings(
                    engineAddr,
                    request.MonitorAddress,
                    request.Config,
                    request.Project,
                    request.Stack,
                    request.Parallel,
                    request.DryRun);

                var deployment = new Deployment(runInfo);
                await deployment.RunInstanceAsync(this._program);
            }
            catch (Exception e) // Use more specific exceptions
            {
                var error = "Inline source runtime error:" + e.ToString();
                return new RunResponse { Error = error };
            }
            finally
            {
                this._logger.LogInformation("Releasing lock with id {0}", contextId);
                this._semaphore.Release();
            }

            return new RunResponse();
        }

        protected virtual void Dispose(bool disposing)
        {
            if (!this._isDisposed)
            {
                if (disposing)
                {
                    this._cts.Dispose();
                    //_semaphore.Dispose(); // This can deadlock if there are calls still waiting for the semaphore.
                }

                this._isDisposed = true;
            }
        }

        public void Dispose()
        {
            // Do not change this code. Put cleanup code in 'Dispose(bool disposing)' method
            Dispose(disposing: true);
            GC.SuppressFinalize(this);
        }
    }
}
