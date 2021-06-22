// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Linq;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Logging;
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

        public override Task<GetRequiredPluginsResponse> GetRequiredPlugins(GetRequiredPluginsRequest request, ServerCallContext context)
        {
            var response = new GetRequiredPluginsResponse();
            return Task.FromResult(response);
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
                _callerContext.Logger,
                engineAddr,
                request.MonitorAddress,
                request.Config,
                request.ConfigSecretKeys,
                request.Project,
                request.Stack,
                request.Parallel,
                request.DryRun);

            using var cts = CancellationTokenSource.CreateLinkedTokenSource(
                this._callerContext.CancellationToken,
                context.CancellationToken);

            var result = await Deployment.RunInlineAsync(
                settings,
                runner => this._callerContext.Program.InvokeAsync(runner, cts.Token))
                .ConfigureAwait(false);

            if (result.ExitCode != 0 || result.ExceptionDispatchInfo != null)
            {
                this._callerContext.ExceptionDispatchInfo = result.ExceptionDispatchInfo;
                return new RunResponse()
                {
                    Bail = true,
                    Error = result.ExceptionDispatchInfo?.SourceException.Message ?? "One or more errors occurred.",
                };
            }

            return new RunResponse();
        }

        public class CallerContext
        {
            public PulumiFn Program { get; }

            public ILogger? Logger { get; }

            public CancellationToken CancellationToken { get; }

            public ExceptionDispatchInfo? ExceptionDispatchInfo { get; set; }

            public CallerContext(
                PulumiFn program,
                ILogger? logger,
                CancellationToken cancellationToken)
            {
                this.Program = program;
                this.Logger = logger;
                this.CancellationToken = cancellationToken;
            }
        }
    }
}
