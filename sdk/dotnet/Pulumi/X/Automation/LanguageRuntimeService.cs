using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
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
        private bool _isDisposed;

        public LanguageRuntimeService(PulumiFn program)
        {
            _program = program;
        }

        public override async Task<RunResponse> Run(RunRequest request, ServerCallContext context)
        {
            var contextId = Guid.NewGuid();
            Console.WriteLine($"Waiting for lock with id {contextId}");
            await _semaphore.WaitAsync();

            Console.WriteLine($"Obtained lock with id {contextId}");
            if (Token.IsCancellationRequested)
            {
                Console.WriteLine($"Releasing lock with id {contextId}");
                _semaphore.Release();
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
                await deployment.RunInstanceAsync(() => _program());
            }
            catch (Exception e) // Use more specific exceptions
            {
                var error = "Inline source runtime error:" + e.ToString();
                return new RunResponse { Error = error };
            }
            finally
            {
                Console.WriteLine($"Releasing lock with id {contextId}");
                _semaphore.Release();
            }

            return new RunResponse();
        }

        protected virtual void Dispose(bool disposing)
        {
            if (!_isDisposed)
            {
                if (disposing)
                {
                    _cts.Dispose();
                    //_semaphore.Dispose(); // This can deadlock if there are calls still waiting for the semaphore.
                }

                _isDisposed = true;
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
