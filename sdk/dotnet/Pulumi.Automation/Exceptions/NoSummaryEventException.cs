// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;

namespace Pulumi.Automation.Exceptions
{
    public sealed class NoSummaryEventException : MissingExpectedEventException
    {
        internal NoSummaryEventException(string? message) : base(nameof(SummaryEvent), message)
        {
        }
    }
}
