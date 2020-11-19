namespace Pulumi.X.Automation.Commands.Exceptions
{
    public class StackNotFoundException : CommandException
    {
        internal StackNotFoundException(CommandResult result)
            : base(nameof(StackNotFoundException), result)
        {
        }
    }
}
