// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// Special exception we throw if we had a problem actually logging a message to the engine
    /// error rpc endpoint. In this case, we have no choice but to tear ourselves down reporting
    /// whatever we can to the console instead.
    /// </summary>
    internal class LogException : Exception
    {
        public LogException(Exception exception)
            : base("Error occurred during logging", exception)
        {
        }
    }
}
