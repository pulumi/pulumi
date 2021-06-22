// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation.Exceptions
{
    public class MissingExpectedEventException : Exception
    {
        public string Name { get; }

        internal MissingExpectedEventException(string name, string? message)
            : base(message)
        {
            Name = name;
        }
    }
}
