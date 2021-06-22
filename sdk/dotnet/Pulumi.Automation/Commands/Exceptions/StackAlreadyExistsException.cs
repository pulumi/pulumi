// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation.Commands.Exceptions
{
    public class StackAlreadyExistsException : CommandException
    {
        internal StackAlreadyExistsException(CommandResult result)
            : base(nameof(StackAlreadyExistsException), result)
        {
        }
    }
}
