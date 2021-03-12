// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Commands.Exceptions;

namespace Pulumi.Automation.Commands
{
    internal class LocalPulumiCmd : IPulumiCmd
    {
        public async Task<CommandResult> RunAsync(
            IEnumerable<string> args,
            string workingDir,
            IDictionary<string, string> additionalEnv,
            Action<string>? onOutput = null,
            Action<string>? onError = null,
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
                    onOutput?.Invoke(@event.Data);
                }
            };

            var standardErrorBuilder = new StringBuilder();
            proc.ErrorDataReceived += (_, @event) =>
            {
                if (@event.Data != null)
                {
                    standardErrorBuilder.AppendLine(@event.Data);
                    onError?.Invoke(@event.Data);
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
                var code = proc.ExitCode;

                var result = new CommandResult(code, standardOutputBuilder.ToString(), standardErrorBuilder.ToString());
                if (code != 0)
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
            return await tcs.Task.ConfigureAwait(false);
        }
    }
}
