// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Reflection;
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

        internal static async Task<InlineDeploymentResult> RunInlineAsync(InlineDeploymentSettings settings, Func<IRunner, Task<int>> runnerFunc)
        {
            var result = new InlineDeploymentResult();

            result.ExitCode = await CreateRunnerAndRunAsync(
                () => new Deployment(settings),
                async runner =>
                {
                    int? exitCode = null;
                    try
                    {
                        exitCode = await runnerFunc(runner).ConfigureAwait(false);

                        // if there was swallowed exceptions from the in-flight tasks we want to either capture
                        // if it is single or re-throw as an aggregate exception if there is more than 1
                        if (runner.SwallowedExceptions.Count == 1)
                        {
                            ExceptionDispatchInfo.Throw(runner.SwallowedExceptions[0]);
                        }
                        else if (runner.SwallowedExceptions.Count > 1)
                        {
                            throw new AggregateException(runner.SwallowedExceptions);
                        }
                    }
                    // because we might be newing a generic, reflection comes in to
                    // construct the instance. And if there is an exception in
                    // the constructor of the user-provided TStack, it will be wrapped
                    // in TargetInvocationException - which is not the exception
                    // we want to throw to the consumer.
                    catch (TargetInvocationException ex) when (ex.InnerException != null)
                    {
                        result.ExceptionDispatchInfo = ExceptionDispatchInfo.Capture(ex.InnerException);
                    }
                    catch (Exception ex)
                    {
                        result.ExceptionDispatchInfo = ExceptionDispatchInfo.Capture(ex);
                    }

                    // return original exit code if we have it, otherwise fail
                    return exitCode ?? -1;
                })
                .ConfigureAwait(false);

            return result;
        }
    }
}
