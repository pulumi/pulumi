// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation.Commands.Exceptions
{
    public class StackNotFoundException : CommandException
    {
        internal StackNotFoundException(CommandResult result)
            : base(nameof(StackNotFoundException), result)
        {
        }
    }
}
