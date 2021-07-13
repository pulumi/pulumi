// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Pulumirpc;

namespace Pulumi
{
    public sealed partial class Deployment
    {
        private class EngineLogger : IEngineLogger
        {
            private readonly object _logGate = new object();
            private readonly IDeploymentInternal _deployment;
            private readonly ILogger _deploymentLogger;
            private readonly IEngine _engine;

            // We serialize all logging tasks so that the engine doesn't hear about them out of order.
            // This is necessary for streaming logs to be maintained in the right order.
            private Task _lastLogTask = Task.CompletedTask;
            private int _errorCount;

            public EngineLogger(IDeploymentInternal deployment, ILogger deploymentLogger, IEngine engine)
            {
                _deployment = deployment;
                _deploymentLogger = deploymentLogger;
                _engine = engine;
            }

            public bool LoggedErrors
            {
                get
                {
                    lock (_logGate)
                    {
                        return _errorCount > 0;
                    }
                }
            }

            /// <summary>
            /// Logs a debug-level message that is generally hidden from end-users.
            /// </summary>
            Task IEngineLogger.DebugAsync(string message, Resource? resource, int? streamId, bool? ephemeral)
            {
                _deploymentLogger.LogDebug(message);
                return LogImplAsync(LogSeverity.Debug, message, resource, streamId, ephemeral);
            }

            /// <summary>
            /// Logs an informational message that is generally printed to stdout during resource
            /// operations.
            /// </summary>
            Task IEngineLogger.InfoAsync(string message, Resource? resource, int? streamId, bool? ephemeral)
            {
                _deploymentLogger.LogInformation(message);
                return LogImplAsync(LogSeverity.Info, message, resource, streamId, ephemeral);
            }

            /// <summary>
            /// Warn logs a warning to indicate that something went wrong, but not catastrophically so.
            /// </summary>
            Task IEngineLogger.WarnAsync(string message, Resource? resource, int? streamId, bool? ephemeral)
            {
                _deploymentLogger.LogWarning(message);
                return LogImplAsync(LogSeverity.Warning, message, resource, streamId, ephemeral);
            }

            /// <summary>
            /// Logs a fatal condition. Consider raising an exception
            /// after calling this method to stop the Pulumi program.
            /// </summary>
            Task IEngineLogger.ErrorAsync(string message, Resource? resource, int? streamId, bool? ephemeral)
                => ErrorAsync(message, resource, streamId, ephemeral);

            private Task ErrorAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            {
                _deploymentLogger.LogError(message);
                return LogImplAsync(LogSeverity.Error, message, resource, streamId, ephemeral);
            }

            private Task LogImplAsync(LogSeverity severity, string message, Resource? resource, int? streamId, bool? ephemeral)
            {
                // Serialize our logging tasks so that streaming logs appear in order.
                Task task;
                lock (_logGate)
                {
                    if (severity == LogSeverity.Error)
                        _errorCount++;

                    // Use a Task.Run here so that we don't end up aggressively running the actual
                    // logging while holding this lock.
                    _lastLogTask = _lastLogTask.ContinueWith(
                        _ => Task.Run(() => LogAsync(severity, message, resource, streamId, ephemeral)),
                        CancellationToken.None, TaskContinuationOptions.None, TaskScheduler.Default).Unwrap();
                    task = _lastLogTask;
                }

                _deployment.Runner.RegisterTask(message, task);
                return task;
            }

            private async Task LogAsync(LogSeverity severity, string message, Resource? resource, int? streamId, bool? ephemeral)
            {
                try
                {
                    var urn = await TryGetResourceUrnAsync(resource).ConfigureAwait(false);

                    await _engine.LogAsync(new LogRequest
                    {
                        Severity = severity,
                        Message = message,
                        Urn = urn,
                        StreamId = streamId ?? 0,
                        Ephemeral = ephemeral ?? false,
                    });
                }
                catch (Exception e)
                {
                    lock (_logGate)
                    {
                        // mark that we had an error so that our top level process quits with an error
                        // code.
                        _errorCount++;
                    }

                    // we have a potential pathological case with logging.  Consider if logging a
                    // message itself throws an error.  If we then allow the error to bubble up, our top
                    // level handler will try to log that error, which can potentially lead to an error
                    // repeating unendingly.  So, to prevent that from happening, we report a very specific
                    // exception that the top level can know about and handle specially.
                    throw new LogException(e);
                }
            }

            private static async Task<string> TryGetResourceUrnAsync(Resource? resource)
            {
                if (resource != null)
                {
                    try
                    {
                        return await resource.Urn.GetValueAsync().ConfigureAwait(false);
                    }
                    catch
                    {
                        // getting the urn for a resource may itself fail.  in that case we don't want to
                        // fail to send an logging message. we'll just send the logging message unassociated
                        // with any resource.
                    }
                }

                return "";
            }
        }
    }
}
