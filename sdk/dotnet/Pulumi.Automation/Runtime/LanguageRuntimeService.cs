// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Linq;
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
