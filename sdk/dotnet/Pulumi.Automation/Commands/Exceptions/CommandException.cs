using System;
using System.Text.RegularExpressions;

namespace Pulumi.Automation.Commands.Exceptions
{
    public class CommandException : Exception
    {
        public string Name { get; }

        internal CommandException(CommandResult result)
            : this(nameof(CommandException), result)
        {
        }

        internal CommandException(string name, CommandResult result)
            : base(result.ToString())
        {
            this.Name = name;
        }

        private static readonly Regex NotFoundRegexPattern = new Regex("no stack named.*found");
        private static readonly Regex AlreadyExistsRegexPattern = new Regex("stack.*already exists");
        private static readonly string ConflictText = "[409] Conflict: Another update is currently in progress.";

        internal static CommandException CreateFromResult(CommandResult result)
            => NotFoundRegexPattern.IsMatch(result.StandardError) ? new StackNotFoundException(result)
            : AlreadyExistsRegexPattern.IsMatch(result.StandardError) ? new StackAlreadyExistsException(result)
            : result.StandardError?.IndexOf(ConflictText) >= 0 ? new ConcurrentUpdateException(result)
            : new CommandException(result);
    }
}
