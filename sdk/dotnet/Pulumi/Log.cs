// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Logging functions that can be called from a .NET application that will be logged to the
    /// <c>Pulumi</c> log stream.  These events will be printed in the terminal while the Pulumi app
    /// runs, and will be available from the Web console afterwards.
    /// </summary>
    public static class Log
    {
        /// <summary>
        /// Logs a debug-level message that is generally hidden from end-users.
        /// </summary>
        public static void Debug(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.DebugAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Logs an informational message that is generally printed to stdout during resource
        /// operations.
        /// </summary>
        public static void Info(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.InfoAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Warn logs a warning to indicate that something went wrong, but not catastrophically so.
        /// </summary>
        public static void Warn(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.WarnAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Error logs a fatal error to indicate that the tool should stop processing resource
        /// operations immediately.
        /// </summary>
        public static void Error(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.ErrorAsync(message, resource, streamId, ephemeral);
    }
}
