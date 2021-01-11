using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Logging;
using Pulumi.X.Automation.Runtime;
using Pulumirpc;

namespace Pulumi.X.Automation
{
    internal class LanguageRuntimeService : LanguageRuntime.LanguageRuntimeBase
    {
        private static readonly SemaphoreSlim Semaphore = new SemaphoreSlim(1, 1);

        // MaxRpcMesageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        public const int MaxRpcMesageSize = 1024 * 1024 * 400;

        private readonly PulumiFn _program;
        private readonly CancellationToken _cancelToken;
        private readonly ILogger<LanguageRuntimeService> _logger;

        public LanguageRuntimeService(
            LanguageRuntimeServiceArgs args,
            ILogger<LanguageRuntimeService> logger)
        {
            this._program = args.Program;
            this._cancelToken = args.CancellationToken;
            this._logger = logger;
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
            await Semaphore.WaitAsync().ConfigureAwait(false);

            this._logger.LogInformation("Obtained lock with id {0}", contextId);
            if (this._cancelToken.IsCancellationRequested // if caller of UpAsync has cancelled
                || context.CancellationToken.IsCancellationRequested) // if CLI has cancelled
            {
                this._logger.LogInformation("Operation cancelled, releasing lock with id {0}", contextId);
                Semaphore.Release();
                return new RunResponse();
            }

            try
            {
                var args = request.Args;
                var engineAddr = args != null && args.Any() ? args[0] : "";

                var settings = new RuntimeSettings(
                    engineAddr,
                    request.MonitorAddress,
                    request.Config,
                    request.Project,
                    request.Stack,
                    request.Parallel,
                    request.DryRun);

                await Deployment.RunInlineAsync(settings, this._program).ConfigureAwait(false);
                Deployment.Instance = null!;
            }
            catch (Exception e) // Use more specific exceptions
            {
                var error = "Inline source runtime error:" + e.ToString();
                return new RunResponse { Error = error };
            }
            finally
            {
                this._logger.LogInformation("Releasing lock with id {0}", contextId);
                Semaphore.Release();
            }

            return new RunResponse();
        }

        public class LanguageRuntimeServiceArgs
        {
            public PulumiFn Program { get; }

            public CancellationToken CancellationToken { get; }

            public LanguageRuntimeServiceArgs(
                PulumiFn program,
                CancellationToken cancellationToken)
            {
                this.Program = program;
                this.CancellationToken = cancellationToken;
            }
        }
    }
}
