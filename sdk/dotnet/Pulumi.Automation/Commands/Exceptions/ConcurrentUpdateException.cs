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
