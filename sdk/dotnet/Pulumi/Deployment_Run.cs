// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Threading.Tasks;
using Serilog;

namespace Pulumi
{
    public partial class Deployment
    {
        public static Task<int> RunAsync(Action action)
            => RunAsync(() =>
            {
                action();
                return ImmutableDictionary<string, object>.Empty;
            });

        public static Task<int> RunAsync(Func<IDictionary<string, object>> func)
            => RunAsync(() => Task.FromResult(func()));

        public static Task<int> RunAsync(Func<Task<IDictionary<string, object>>> func)
        {
            // Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();

            Serilog.Log.Debug("Deployment.Run called.");
            if (_instance != null)
            {
                throw new NotSupportedException("Deployment.Run can only be called a single time.");
            }

            Serilog.Log.Debug("Creating new Deployment.");
            Instance = new Deployment();
            return Instance.RunWorkerAsync(func);
        }

        private Task<int> RunWorkerAsync(Func<Task<IDictionary<string, object>>> func)
        {
            var stack = new Stack(func);
            RegisterTask("User program code.", stack.Outputs.DataTask);
            return WhileRunningAsync();
        }

        internal void RegisterTask(string description, Task task)
        {
            lock (_taskToDescription)
            {
                _taskToDescription.Add(task, description);//.Enqueue((description, task));
            }
        }

        // Keep track if we already logged the information about an unhandled error to the user..  If
        // so, we end with a different exit code.  The language host recognizes this and will not print
        // any further messages to the user since we already took care of it.
        //
        // 32 was picked so as to be very unlikely to collide with any other error codes.
        private const int _processExitedAfterLoggingUserActionableMessage = 32;

        private readonly Dictionary<Task, string> _taskToDescription = new Dictionary<Task, string>();

        private async Task<int> WhileRunningAsync()
        {
            var tasks = new List<Task>();

            // Keep looping as long as there are outstanding tasks that are still running.
            while (true)
            {
                tasks.Clear();
                lock (_taskToDescription)
                {
                    if (_taskToDescription.Count == 0)
                    {
                        break;
                    }

                    // grab all the tasks we currently have running.
                    tasks.AddRange(_taskToDescription.Keys);
                }

                // Now, wait for one of them to finish.
                var task = await Task.WhenAny(tasks).ConfigureAwait(false);
                string description;
                lock (_taskToDescription)
                {
                    // once finished, remove it from the set of tasks that are running.
                    description = _taskToDescription[task];
                    _taskToDescription.Remove(task);
                }

                try
                {
                    // Now actually await that completed task so that we will realize any exceptions
                    // is may have thrown.
                    await task.ConfigureAwait(false);
                }
                catch (Exception e)
                {
                    // if it threw, report it as necessary, then quit.
                    return await HandleExceptionAsync(e).ConfigureAwait(false);
                }
            }

            // there were no more tasks we were waiting on.  Quit out, reporting if we had any
            // errors or not.
            return HasErrors ? 1 : 0;
        }

        private async Task<int> HandleExceptionAsync(Exception exception)
        {
            if (exception is LogException)
            {
                // We got an error while logging itself.  Nothing to do here but print some errors
                // and fail entirely.
                Serilog.Log.Error(exception, "Error occurred trying to send logging message to engine.");
                await Console.Error.WriteLineAsync("Error occurred trying to send logging message to engine:\n" + exception).ConfigureAwait(false);
                return 1;
            }

            // For the rest of the issue we encounter log the problem to the error stream. if we
            // successfully do this, then return with a special error code stating as such so that
            // our host doesn't print out another set of errors.
            //
            // Note: if these logging calls fail, they will just end up bubbling up an exception
            // that will be caught by nothing.  This will tear down the actual process with a
            // non-zero error which our host will handle properly.
            if (exception is RunException)
            {
                // Always hide the stack for RunErrors.
                await ErrorAsync(exception.Message).ConfigureAwait(false);
            }
            else if (exception is ResourceException resourceEx)
            {
                var message = resourceEx.HideStack
                    ? resourceEx.Message
                    : resourceEx.ToString();
                await ErrorAsync(message, resourceEx.Resource).ConfigureAwait(false);
            }
            else
            {
                var location = System.Reflection.Assembly.GetEntryAssembly()?.Location;
                await ErrorAsync(
$@"Running program '{location}' failed with an unhandled exception:
{exception.ToString()}").ConfigureAwait(false);
            }

            Serilog.Log.Debug("Wrote last error.  Returning from program.");
            return _processExitedAfterLoggingUserActionableMessage;
        }
    }
}
