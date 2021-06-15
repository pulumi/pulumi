// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using CliWrap;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;

namespace Pulumi.Automation.Commands
{
    internal class LocalPulumiCmd : IPulumiCmd
    {

        public async Task<CommandResult> RunAsync(
            IList<string> args,
            string workingDir,
            IDictionary<string, string?> additionalEnv,
            Action<string>? onStandardOutput = null,
            Action<string>? onStandardError = null,
            Action<EngineEvent>? onEngineEvent = null,
            CancellationToken cancellationToken = default)
        {
            if (onEngineEvent != null)
            {
                var commandName = SanitizeCommandName(args.FirstOrDefault());
                using var eventLogFile = new EventLogFile(commandName);
                using var eventLogWatcher = new EventLogWatcher(eventLogFile.FilePath, onEngineEvent, cancellationToken);
                try
                {
                    return await RunAsyncInner(args, workingDir, additionalEnv, onStandardOutput, onStandardError, eventLogFile, cancellationToken);
                } finally {
                    await eventLogWatcher.Stop();
                }
            }
            return await RunAsyncInner(args, workingDir, additionalEnv, onStandardOutput, onStandardError, eventLogFile: null, cancellationToken);
        }

        private async Task<CommandResult> RunAsyncInner(
            IList<string> args,
            string workingDir,
            IDictionary<string, string?> additionalEnv,
            Action<string>? onStandardOutput = null,
            Action<string>? onStandardError = null,
            EventLogFile? eventLogFile = null,
            CancellationToken cancellationToken = default)
        {
            var stdOutBuffer = new StringBuilder();
            var stdOutPipe = PipeTarget.ToStringBuilder(stdOutBuffer);
            if (onStandardOutput != null)
            {
                stdOutPipe = PipeTarget.Merge(stdOutPipe, PipeTarget.ToDelegate(onStandardOutput));
            }

            var stdErrBuffer = new StringBuilder();
            var stdErrPipe = PipeTarget.ToStringBuilder(stdErrBuffer);
            if (onStandardError != null)
            {
                stdErrPipe = PipeTarget.Merge(stdErrPipe, PipeTarget.ToDelegate(onStandardError));
            }

            var pulumiCmd = Cli.Wrap("pulumi")
                .WithArguments(PulumiArgs(args, eventLogFile), escape: true)
                .WithWorkingDirectory(workingDir)
                .WithEnvironmentVariables(PulumiEnvironment(additionalEnv, debugCommands: eventLogFile != null))
                .WithStandardOutputPipe(stdOutPipe)
                .WithStandardErrorPipe(stdErrPipe)
                .WithValidation(CommandResultValidation.None); // we check non-0 exit code ourselves

            var pulumiCmdResult = await pulumiCmd.ExecuteAsync(cancellationToken);

            var result = new CommandResult(
                pulumiCmdResult.ExitCode,
                standardOutput: stdOutBuffer.ToString(),
                standardError: stdErrBuffer.ToString());

            if (pulumiCmdResult.ExitCode != 0)
            {
                throw CommandException.CreateFromResult(result);
            }

            return result;
        }

        private static IReadOnlyDictionary<string, string?> PulumiEnvironment(IDictionary<string, string?> additionalEnv, bool debugCommands)
        {
            var env = new Dictionary<string, string?>(additionalEnv);

            if (debugCommands)
            {
                // Required for event log
                // We add it after the provided env vars to ensure it is set to true
                env["PULUMI_DEBUG_COMMANDS"] = "true";
            }

            return env;
        }

        private static IList<string> PulumiArgs(IList<string> args, EventLogFile? eventLogFile)
        {
            // all commands should be run in non-interactive mode.
            // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
            if (!args.Contains("--non-interactive"))
            {
                args = args.Concat(new[] { "--non-interactive" }).ToList();
            }

            if (eventLogFile != null)
            {
                args = args.Concat(new[] { "--event-log", eventLogFile.FilePath }).ToList();
            }

            return args;
        }

        private static string SanitizeCommandName(string? firstArgument)
        {
            var alphaNumWord = new Regex(@"^[-A-Za-z0-9_]{1,20}$");
            if (firstArgument == null)
            {
                return "event-log";
            }
            return alphaNumWord.IsMatch(firstArgument) ? firstArgument : "event-log";
        }

        private sealed class EventLogFile : IDisposable
        {
            public string FilePath { get; }

            public EventLogFile(string command)
            {
                var logDir = Path.Combine(Path.GetTempPath(), $"automation-logs-{command}-{Path.GetRandomFileName()}");
                Directory.CreateDirectory(logDir);
                this.FilePath = Path.Combine(logDir, "eventlog.txt");
            }

            public void Dispose()
            {
                var dir = Path.GetDirectoryName(this.FilePath);
                try
                {
                    Directory.Delete(dir, recursive: true);
                }
                catch (Exception e)
                {
                    // allow graceful exit if for some reason
                    // we're not able to delete the directory
                    // will rely on OS to clean temp directory
                    // in this case.
                    Trace.TraceWarning("Ignoring exception during cleanup of {0} folder: {1}", dir, e);
                }
            }
        }
    }
}
