using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.X.Automation.Commands.Exceptions;

namespace Pulumi.X.Automation.Commands
{
    internal static class PulumiCmd
    {
        public static Task<CommandResult> RunAsync(
            IEnumerable<string> args,
            string workingDir,
            IDictionary<string, string> additionalEnv,
            Action<string>? onOutput = null,
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

            var tcs = new TaskCompletionSource<CommandResult>();
            var proc = new Process
            {
                EnableRaisingEvents = true
            };

            proc.StartInfo.FileName = "pulumi";
            proc.StartInfo.WorkingDirectory = workingDir;
            proc.StartInfo.CreateNoWindow = true;
            proc.StartInfo.UseShellExecute = false;
            proc.StartInfo.RedirectStandardError = true;
            proc.StartInfo.RedirectStandardOutput = true;

            foreach (var arg in completeArgs)
                proc.StartInfo.ArgumentList.Add(arg);

            foreach (var pair in env)
                proc.StartInfo.Environment[pair.Key] = pair.Value;

            proc.OutputDataReceived += (_, @event) =>
            {
                if (@event.Data != null)
                    onOutput?.Invoke(@event.Data);
            };

            var cancelRegistration = cancellationToken.Register(() =>
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

            proc.Exited += async (_, @event) =>
            {
                var code = proc.ExitCode;
                var stdOut = await proc.StandardOutput.ReadToEndAsync();
                var stdErr = await proc.StandardError.ReadToEndAsync();

                var result = new CommandResult(code, stdOut, stdErr);
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

            // need to ensure that the cancellation registration is *always* disposed
            // regardless of the status (faulted, cancelled, completed) of the process task.
            // originally had this in proc.Exited but that introduces race condition
            // if task is cancelled before process starts. this allows us to cleanup process too.
            var cleanupWrapperTask = tcs.Task.ContinueWith(async processTask =>
            {
                try
                {
                    return await processTask;
                }
                finally
                {
                    cancelRegistration.Dispose();

                    if (proc.HasExited)
                        proc.Dispose();
                }
            }, TaskContinuationOptions.None);

            proc.Start();
            return cleanupWrapperTask.Unwrap();
        }
    }
}
