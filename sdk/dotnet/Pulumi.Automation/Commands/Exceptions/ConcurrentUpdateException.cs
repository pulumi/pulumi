// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation.Commands.Exceptions
{
    public sealed class ConcurrentUpdateException : CommandException
    {
        internal ConcurrentUpdateException(CommandResult result)
            : base(nameof(ConcurrentUpdateException), result)
        {
        }
    }
}
