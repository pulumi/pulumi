using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using Pulumi.X.Automation.Runtime;
using Pulumirpc;

namespace Pulumi.X.Automation
{
    internal class LanguageRuntimeService : LanguageRuntime.LanguageRuntimeBase
    {
        // MaxRpcMesageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        public const int MaxRpcMesageSize = 1024 * 1024 * 400;

        private readonly PulumiFn _program;
        private readonly IEnumerable<Assembly>? _resourcePackageAssemblies;
        private readonly CancellationToken _cancelToken;

        public LanguageRuntimeService(LanguageRuntimeServiceArgs args)
        {
            this._program = args.Program;
            this._resourcePackageAssemblies = args.ResourcePackageAssemblies;
            this._cancelToken = args.CancellationToken;
        }

        public override Task<GetRequiredPluginsResponse> GetRequiredPlugins(GetRequiredPluginsRequest request, ServerCallContext context)
        {
            var response = new GetRequiredPluginsResponse();
            return Task.FromResult(response);
        }

        public override async Task<RunResponse> Run(RunRequest request, ServerCallContext context)
        {
            if (this._cancelToken.IsCancellationRequested // if caller of UpAsync has cancelled
                || context.CancellationToken.IsCancellationRequested) // if CLI has cancelled
            {
                return new RunResponse();
            }

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

            await Deployment.RunInlineAsync(settings, this._program, this._resourcePackageAssemblies).ConfigureAwait(false);
            return new RunResponse();
        }

        public class LanguageRuntimeServiceArgs
        {
            public PulumiFn Program { get; }

            public IEnumerable<Assembly>? ResourcePackageAssemblies { get; }

            public CancellationToken CancellationToken { get; }

            public LanguageRuntimeServiceArgs(
                PulumiFn program,
                IEnumerable<Assembly>? resourcePackageAssemblies,
                CancellationToken cancellationToken)
            {
                this.Program = program;
                this.ResourcePackageAssemblies = resourcePackageAssemblies;
                this.CancellationToken = cancellationToken;
            }
        }
    }
}
