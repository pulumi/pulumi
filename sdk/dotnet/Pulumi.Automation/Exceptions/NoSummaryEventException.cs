// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation.Exceptions
{
    public sealed class NoSummaryEventException : MissingExpectedEventException
    {
        internal NoSummaryEventException(string? message) : base(nameof(Events.SummaryEvent), message)
        {
        }
    }
}
