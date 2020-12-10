using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Grpc.Core;
using Pulumi.X.Automation.Runtime;
using Pulumirpc;

namespace Pulumi.X.Automation
{
    internal class LanguageServer<T> : LanguageRuntime.LanguageRuntimeBase
    {
        // MaxRpcMesageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        public const int MaxRpcMesageSize = 1024 * 1024 * 400;

        private readonly PulumiFn _program;

        private bool _isRunning;

        public LanguageServer(PulumiFn program, ServerCallContext context)
        {
            _program = program;
            _isRunning = false;
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
            _isRunning = true;

            var args = request.Args;
            var engineAddr = args != null && args.Any() ? args[0] : "";

            var runInfo = new RunInfo(
                engineAddr,
                request.MonitorAddress,
                request.Config,
                request.Project,
                request.Stack,
                request.Parallel);

            try
            {
                // TODO: Set and pass config

                await Deployment.RunAsync(() => _program());
            }
            catch (Exception e)
            {
                return new RunResponse { Error = e.ToString() };
            }

            return new RunResponse();
        }
    }
}
