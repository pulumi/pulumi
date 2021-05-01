// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using Pulumirpc;

namespace Pulumi.Automation
{
    internal class LanguageRuntimeService : LanguageRuntime.LanguageRuntimeBase
    {
        // MaxRpcMesageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        public const int MaxRpcMesageSize = 1024 * 1024 * 400;

        private readonly CallerContext _callerContext;

        public LanguageRuntimeService(CallerContext callerContext)
        {
            this._callerContext = callerContext;
        }

        public override async Task<GetRequiredPluginsResponse> GetRequiredPlugins(GetRequiredPluginsRequest request, ServerCallContext context)
        {
            var response = new GetRequiredPluginsResponse();
            var assembly = this._callerContext.Program.DiscoveryAssembly;

            // if discovery assembly is null, assuming consumer installed plugins manually
            if (assembly is null)
                return response;

            var pulumiAssemblyNames = assembly.GetReferencedAssemblies().Where(x => x.Name != null && x.Name.StartsWith("Pulumi.")).ToArray();
            foreach (var assemblyName in pulumiAssemblyNames)
            {
                var pulumiAssembly = Assembly.Load(assemblyName);

                // get version.txt
                // this part would probably be less error prone if there was an assembly attribute available
                // instead of using the embedded version.txt resource
                // pulumiAssembly.GetCustomAttribute<ResourcePluginAttribute>();
                var resources = pulumiAssembly.GetManifestResourceNames();
                var versionResource = resources.FirstOrDefault(x => x.EndsWith("version.txt"));
                if (versionResource is null)
                {
                    // no version.txt so is not resource plugin
                    continue;
                }

                using var versionStream = pulumiAssembly.GetManifestResourceStream(versionResource);
                if (versionStream is null)
                {
                    // we've already verified that version.txt should be present so throw here?
                    throw new ApplicationException($"Unable to load version.txt from {assemblyName.FullName}");
                }

                var versionReader = new StreamReader(versionStream);
                var versionText = await versionReader.ReadToEndAsync().ConfigureAwait(false);

                // ToLower() everything after "Pulumi." to determine plugin name
                var pluginName = assemblyName.Name!.Substring("Pulumi.".Length).ToLower();
                var version = versionText.Trim();
                var parts = versionText.Split('\n', StringSplitOptions.RemoveEmptyEntries)
                    .Select(x => x.Trim())
                    .ToArray();

                // if version.txt is 2 lines than it is specifying plugin name explicitly
                if (parts.Length is 2)
                {
                    pluginName = parts[0];
                    version = parts[1];
                }

                // pre-pend 'v' if it isn't present on version string
                if (!version.StartsWith('v'))
                    version = $"v{version}";

                response.Plugins.Add(new PluginDependency
                {
                    Name = pluginName,
                    Version = version,
                    Kind = "resource",
                });
            }

            return response;
        }

        public override async Task<RunResponse> Run(RunRequest request, ServerCallContext context)
        {
            if (this._callerContext.CancellationToken.IsCancellationRequested // if caller of UpAsync has cancelled
                || context.CancellationToken.IsCancellationRequested) // if CLI has cancelled
            {
                return new RunResponse();
            }

            var args = request.Args;
            var engineAddr = args != null && args.Any() ? args[0] : "";

            var settings = new InlineDeploymentSettings(
                engineAddr,
                request.MonitorAddress,
                request.Config,
                request.Project,
                request.Stack,
                request.Parallel,
                request.DryRun);

            using var cts = CancellationTokenSource.CreateLinkedTokenSource(
                this._callerContext.CancellationToken,
                context.CancellationToken);

            this._callerContext.ExceptionDispatchInfo = await Deployment.RunInlineAsync(
                settings,
                runner => this._callerContext.Program.InvokeAsync(runner, cts.Token))
                .ConfigureAwait(false);

            return new RunResponse();
        }

        public class CallerContext
        {
            public PulumiFn Program { get; }

            public CancellationToken CancellationToken { get; }

            public ExceptionDispatchInfo? ExceptionDispatchInfo { get; set; }

            public CallerContext(
                PulumiFn program,
                CancellationToken cancellationToken)
            {
                this.Program = program;
                this.CancellationToken = cancellationToken;
            }
        }
    }
}
