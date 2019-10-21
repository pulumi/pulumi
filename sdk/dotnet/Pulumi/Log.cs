// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi
{
    public static class Log
    {
        /// <summary>
        /// Logs a debug-level message that is generally hidden from end-users.
        /// </summary>
        public static void Debug(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.Instance.DebugAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Logs an informational message that is generally printed to stdout during resource
        /// operations.
        /// </summary>
        public static void Info(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.Instance.InfoAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Warn logs a warning to indicate that something went wrong, but not catastrophically so.
        /// </summary>
        public static void Warn(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.Instance.WarnAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Error logs a fatal error to indicate that the tool should stop processing resource
        /// operations immediately.
        /// </summary>
        public static void Error(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.Instance.ErrorAsync(message, resource, streamId, ephemeral);
    }
}
