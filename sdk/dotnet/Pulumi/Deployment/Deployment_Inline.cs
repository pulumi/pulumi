// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Runtime.ExceptionServices;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace Pulumi
{
    public partial class Deployment
    {
        private Deployment(InlineDeploymentSettings settings)
        {
            if (settings is null)
                throw new ArgumentNullException(nameof(settings));

            _projectName = settings.Project;
            _stackName = settings.Stack;
            _isDryRun = settings.IsDryRun;
            SetAllConfig(settings.Config, settings.ConfigSecretKeys);

            if (string.IsNullOrEmpty(settings.MonitorAddr)
                || string.IsNullOrEmpty(settings.EngineAddr)
                || string.IsNullOrEmpty(_projectName)
                || string.IsNullOrEmpty(_stackName))
            {
                throw new InvalidOperationException("Inline execution was not provided the necessary parameters to run the Pulumi engine.");
            }

            var deploymentLogger = settings.Logger ?? CreateDefaultLogger();

            deploymentLogger.LogDebug("Creating deployment engine");
            Engine = new GrpcEngine(settings.EngineAddr);
            deploymentLogger.LogDebug("Created deployment engine");

            deploymentLogger.LogDebug("Creating deployment monitor");
            Monitor = new GrpcMonitor(settings.MonitorAddr);
            deploymentLogger.LogDebug("Created deployment monitor");

            _runner = new Runner(this, deploymentLogger);
            _logger = new EngineLogger(this, deploymentLogger, Engine);
        }

        internal static async Task<InlineDeploymentResult> RunInlineAsync(InlineDeploymentSettings settings, Func<IRunner, Task<InlineDeploymentResult>> func)
        {
            var result = new InlineDeploymentResult();

            result.ExitCode = await CreateRunnerAndRunAsync(
                () => new Deployment(settings),
                async runner =>
                {
                    InlineDeploymentResult? innerResult = null;
                    try
                    {
                        innerResult = await func(runner).ConfigureAwait(false);

                        // if the inner result has EDI than there was an exception present in the primary context
                        // so we will prioritize throwing that exception over exceptions thrown from in-flight tasks
                        if (innerResult.ExceptionDispatchInfo != null)
                        {
                            result.ExceptionDispatchInfo = innerResult.ExceptionDispatchInfo;
                        }

                        // if there was swallowed exceptions from the in-flight tasks we want to either capture
                        // if it is single or re-throw as an aggregate exception if there is more than 1
                        else if (runner.SwallowedExceptions.Count == 1)
                        {
                            result.ExceptionDispatchInfo = ExceptionDispatchInfo.Capture(runner.SwallowedExceptions[0]);
                        }
                        else if (runner.SwallowedExceptions.Count > 1)
                        {
                            throw new AggregateException(runner.SwallowedExceptions);
                        }

                        return innerResult.ExitCode;
                    }
                    catch (AggregateException ex)
                    {
                        result.ExceptionDispatchInfo = ExceptionDispatchInfo.Capture(ex);

                        // return original exit code if we have it, otherwise fail
                        return innerResult?.ExitCode ?? -1;
                    }
                })
                .ConfigureAwait(false);

            return result;
        }
    }
}
