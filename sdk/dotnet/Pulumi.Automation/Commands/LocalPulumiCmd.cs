// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Commands.Exceptions;
using Pulumi.Automation.Events;

namespace Pulumi.Automation.Commands
{
    internal class LocalPulumiCmd : IPulumiCmd
    {
        public async Task<CommandResult> RunAsync(
            IEnumerable<string> args,
            string workingDir,
            IDictionary<string, string> additionalEnv,
            Action<string>? onStandardOutput = null,
            Action<string>? onStandardError = null,
            Action<EngineEvent>? onEngineEvent = null,
            CancellationToken cancellationToken = default)
        {
            // all commands should be run in non-interactive mode.
            // this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
            var completeArgs = args.Concat(new[] { "--non-interactive" });

            var env = new Dictionary<string, string>();

            foreach (var element in Environment.GetEnvironmentVariables())
            {
                if (element is KeyValuePair<string, object> pair
                    && pair.Value is string valueStr)
                    env[pair.Key] = valueStr;
            }

            foreach (var pair in additionalEnv)
                env[pair.Key] = pair.Value;

            string? eventLogFile = null;
            EventLogWatcher? eventLogWatcher = null;

            if (onEngineEvent != null)
            {
                // Required for event log
                // We add it after the provided env vars to ensure it is set to true
                env["PULUMI_DEBUG_COMMANDS"] = "true";

                eventLogFile = CreateEventLogFile(completeArgs.FirstOrDefault() ?? "event-log");
                eventLogWatcher = new EventLogWatcher(eventLogFile, onEngineEvent, cancellationToken);

                completeArgs = completeArgs.Concat(new[] { "--event-log", eventLogFile });
            }

            using var proc = new Process
            {
                EnableRaisingEvents = true,
                StartInfo = new ProcessStartInfo
                {
                    FileName = "pulumi",
                    WorkingDirectory = workingDir,
                    CreateNoWindow = true,
                    UseShellExecute = false,
                    RedirectStandardError = true,
                    RedirectStandardOutput = true,
                },
            };

            foreach (var arg in completeArgs)
                proc.StartInfo.ArgumentList.Add(arg);

            foreach (var pair in env)
                proc.StartInfo.Environment[pair.Key] = pair.Value;

            var standardOutputBuilder = new StringBuilder();
            proc.OutputDataReceived += (_, @event) =>
            {
                if (@event.Data != null)
                {
                    standardOutputBuilder.AppendLine(@event.Data);
                    onStandardOutput?.Invoke(@event.Data);
                }
            };

            var standardErrorBuilder = new StringBuilder();
            proc.ErrorDataReceived += (_, @event) =>
            {
                if (@event.Data != null)
                {
                    standardErrorBuilder.AppendLine(@event.Data);
                    onStandardError?.Invoke(@event.Data);
                }
            };

            var tcs = new TaskCompletionSource<CommandResult>();
            using var cancelRegistration = cancellationToken.Register(() =>
            {
                // if the process has already exited than let's
                // just let it set the result on the task
                if (proc.HasExited || tcs.Task.IsCompleted)
                    return;

                // setting it cancelled before killing so there
                // isn't a race condition to the proc.Exited event
                tcs.TrySetCanceled(cancellationToken);

                try
                {
                    proc.Kill();
                }
                catch
                {
                    // in case the process hasn't started yet
                    // or has already terminated
                }
            });

            proc.Exited += (_, @event) =>
            {
                // this seems odd, since the exit event has been triggered, but
                // the exit event being triggered does not mean that the async
                // output stream handlers have ran to completion. this method
                // doesn't exit until they have, at which point we can be sure
                // we have captured the output in its entirety.
                // note that if we were to pass an explicit wait time to this
                // method it would not wait for the stream handlers.
                // see: https://github.com/dotnet/runtime/issues/18789
                proc.WaitForExit();

                var result = new CommandResult(
                    proc.ExitCode,
                    standardOutputBuilder.ToString(),
                    standardErrorBuilder.ToString());

                if (proc.ExitCode != 0)
                {
                    var ex = CommandException.CreateFromResult(result);
                    tcs.TrySetException(ex);
                }
                else
                {
                    tcs.TrySetResult(result);
                }
            };

            proc.Start();
            proc.BeginOutputReadLine();
            proc.BeginErrorReadLine();

            try
            {
                return await tcs.Task.ConfigureAwait(false);
            }
            finally
            {
                // If proc.HasExited is false here, it likely means that cancellation was requested.
                // We want to do best effort for ensuring the process exits
                // before removing the event log watcher and the event log file
                // in case the process was still writing into the event log file
                if (!proc.HasExited)
                {
                    proc.WaitForExit();
                }

                if (eventLogWatcher != null)
                {
                    await eventLogWatcher.Stop().ConfigureAwait(false);
                    eventLogWatcher.Dispose();
                }

                if (!string.IsNullOrWhiteSpace(eventLogFile))
                {
                    try
                    {
                         Directory.Delete(Path.GetDirectoryName(eventLogFile), recursive: true);
                    }
                    catch
                    {
                        // allow graceful exit if for some reason
                        // we're not able to delete the directory
                        // will rely on OS to clean temp directory
                        // in this case.
                    }
                }
            }

            static string CreateEventLogFile(string command)
            {
                var logDir = Path.Combine(Path.GetTempPath(), $"automation-logs-{command}-{Path.GetRandomFileName()}");
                Directory.CreateDirectory(logDir);
                return Path.Combine(logDir, "eventlog.txt");
            }
        }
    }
}
