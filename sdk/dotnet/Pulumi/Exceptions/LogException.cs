// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    internal class LogException : Exception
    {
        public LogException(Exception exception)
            : base("Error occurred during logging", exception)
        {
        }
    }
}
