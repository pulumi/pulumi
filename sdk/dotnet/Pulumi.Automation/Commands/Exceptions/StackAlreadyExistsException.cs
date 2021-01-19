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
