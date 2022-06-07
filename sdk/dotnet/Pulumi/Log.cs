// Copyright 2016-2019, Pulumi Corporation

using System;

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
        /// Logs a warning to indicate that something went wrong, but not catastrophically so.
        /// </summary>
        public static void Warn(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.WarnAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Logs a fatal condition. Consider raising an exception
        /// after calling Error to stop the Pulumi program.
        /// </summary>
        public static void Error(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Deployment.InternalInstance.Logger.ErrorAsync(message, resource, streamId, ephemeral);

        /// <summary>
        /// Logs an exception. Consider raising the exception after
        /// calling this method to stop the Pulumi program.
        /// </summary>
        public static void Exception(Exception exception, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => Error(exception.ToString(), resource, streamId, ephemeral);
    }
}
