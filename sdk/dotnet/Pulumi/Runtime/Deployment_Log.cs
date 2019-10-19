// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
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
        public void Debug(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Debug(message);
            Log(LogSeverity.Debug, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Logs an informational message that is generally printed to stdout during resource
        /// operations.
        /// </summary>
        public void Info(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Information(message);
            Log(LogSeverity.Info, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Warn logs a warning to indicate that something went wrong, but not catastrophically so.
        /// </summary>
        public void Warn(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Warning(message);
            Log(LogSeverity.Warning, message, resource, streamId, ephemeral);
        }

        /// <summary>
        /// Error logs a fatal error to indicate that the tool should stop processing resource
        /// operations immediately.
        /// </summary>
        public void Error(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
        {
            Serilog.Log.Error(message);
            Log(LogSeverity.Error, message, resource, streamId, ephemeral);
        }

        private void Log(LogSeverity severity, string message, Resource? resource, int? streamId, bool? ephemeral)
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
                    _ => Task.Run(() => LogAsync(severity, message, resource, streamId, ephemeral))).Unwrap();
                task = _lastLogTask;
            }

            RegisterTask($"Log: {severity}: {message}", task);
        }

        private async Task LogAsync(LogSeverity severity, string message, Resource? resource, int? streamId, bool? ephemeral)
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
    }
}
