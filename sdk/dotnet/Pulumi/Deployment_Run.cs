// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    public partial class Deployment
    {
        public static Task<int> Run(Action action)
            => Run(() =>
            {
                action();
                return ImmutableDictionary<string, object>.Empty;
            });

        public static Task<int> Run(Func<IDictionary<string, object>> func)
            => Run(() => Task.FromResult(func()));

        public static Task<int> Run(Func<Task<IDictionary<string, object>>> func)
        {
            Serilog.Log.Debug("Deployment.Run called.");
            if (Instance != null)
            {
                throw new NotSupportedException("Deployment.Run can only be called a single time.");
            }

            Serilog.Log.Debug("Creating new Deployment.");
            Instance = new Deployment();
            return Instance.RunWorker(func);
        }

        private Task<int> RunWorker(Func<Task<IDictionary<string, object>>> func)
        {
            var stack = new Stack(func);
            RegisterTask("User program code.", stack.Outputs.DataTask);
            return WhileRunning();
        }

        internal void RegisterTask(string description, Task task)
        {
            lock (_tasks)
            {
                _tasks.Enqueue((description, task));
            }
        }

        // Keep track if we already logged the information about an unhandled error to the user..  If
        // so, we end with a different exit code.  The language host recognizes this and will not print
        // any further messages to the user since we already took care of it.
        //
        // 32 was picked so as to be very unlikely to collide with any other error codes.
        private const int _processExitedAfterLoggingUserActionableMessage = 32;

        private async Task<int> WhileRunning()
        {
            while (true)
            {
                string description;
                Task task;
                lock (_tasks)
                {
                    if (_tasks.Count == 0)
                    {
                        break;
                    }

                    (description, task) = _tasks.Dequeue();
                }

                Serilog.Log.Debug("Deployment awaiting: " + description);

                try
                {
                    await task.ConfigureAwait(false);
                }
                catch (Exception e)
                {
                    return await HandleExceptionAsync(e).ConfigureAwait(false);
                }
            }

            return HasErrors ? 1 : 0;
        }

        private async Task<int> HandleExceptionAsync(Exception exception)
        {
            if (exception is LogException)
            {
                // We got an error while logging itself.  Nothing to do here but print some errors
                // and fail entirely.
                Serilog.Log.Error(exception, "Error occurred trying to send logging message to engine.");
                Console.Error.WriteLine("Error occurred trying to send logging message to engine:\n" + exception);
                return 1;
            }

            if (exception is RunException)
            {
                // Always hide the stack for RunErrors.
                await Error(exception.Message).ConfigureAwait(false);
            }
            else if (exception is ResourceException resourceEx)
            {
                var message = resourceEx.HideStack
                    ? resourceEx.Message
                    : resourceEx.ToString();
                await Error(message, resourceEx.Resource).ConfigureAwait(false);
            }
            else
            {
                var location = System.Reflection.Assembly.GetEntryAssembly().Location;
                await Error(
$@"Running program '{location}' failed with an unhandled exception:
{exception.ToString()}");
            }

            return _processExitedAfterLoggingUserActionableMessage;
        }
    }
}
