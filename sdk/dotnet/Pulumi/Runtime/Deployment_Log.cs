// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Threading;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public sealed partial class Deployment
    {
        private readonly object _logGate = new object();
        // We serialize all logging tasks so that the engine doesn't hear about them out of order.
        // This is necessary for streaming logs to be maintained in the right order.
        private Task _lastLogTask = Task.CompletedTask;
        private int _errorCount;

        internal bool HasErrors
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
        internal Task DebugAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Debug(message);
            return LogImplAsync(LogSeverity.Debug, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Logs an informational message that is generally printed to stdout during resource
        /// operations.
        /// </summary>
        internal Task InfoAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Information(message);
            return LogImplAsync(LogSeverity.Info, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Warn logs a warning to indicate that something went wrong, but not catastrophically so.
        /// </summary>
        internal Task WarnAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Warning(message);
            return LogImplAsync(LogSeverity.Warning, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Error logs a fatal error to indicate that the tool should stop processing resource
        /// operations immediately.
        /// </summary>
        internal Task ErrorAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Error(message);
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

            RegisterTask($"Log: {severity}: {message}", task);
            return task;
        }

        private async Task LogAsync(LogSeverity severity, string message, Resource? resource, int? streamId, bool? ephemeral)
        {
            try
            {
                var urn = resource == null
                    ? new Urn("")
                    : await resource.Urn.GetValueAsync().ConfigureAwait(false);

                await Engine.LogAsync(new LogRequest
                {
                    Severity = severity,
                    Message = message,
                    Urn = urn.Value,
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
    }
}
