// Copyright 2016-2021, Pulumi Corporation

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

        private static readonly Regex _notFoundRegexPattern = new Regex("no stack named.*found");
        private static readonly Regex _alreadyExistsRegexPattern = new Regex("stack.*already exists");
        private static readonly string _conflictText = "[409] Conflict: Another update is currently in progress.";

        internal static CommandException CreateFromResult(CommandResult result)
            => _notFoundRegexPattern.IsMatch(result.StandardError) ? new StackNotFoundException(result)
            : _alreadyExistsRegexPattern.IsMatch(result.StandardError) ? new StackAlreadyExistsException(result)
            : result.StandardError.IndexOf(_conflictText, StringComparison.Ordinal) >= 0 ? new ConcurrentUpdateException(result)
            : new CommandException(result);
    }
}
