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
            // Serilog.Log.Logger = new LoggerConfiguration().MinimumLevel.Debug().WriteTo.Console().CreateLogger();

            Serilog.Log.Debug("Deployment.Run called.");
            if (_instance != null)
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

        private async Task<int> WhileRunning()
        {
            while (true)
            {
                Task[] tasks;
                lock (_taskToDescription)
                {
                    if (_taskToDescription.Count == 0)
                    {
                        break;
                    }

                    tasks = _taskToDescription.Keys.ToArray();
                }

                var task = await Task.WhenAny(tasks).ConfigureAwait(false);
                string description;
                lock (_taskToDescription)
                {
                    description = _taskToDescription[task];
                    _taskToDescription.Remove(task);
                }

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
                var location = System.Reflection.Assembly.GetEntryAssembly()?.Location;
                await Error(
$@"Running program '{location}' failed with an unhandled exception:
{exception.ToString()}");
            }

            Serilog.Log.Debug("Wrote last error.  Returning from program.");
            return _processExitedAfterLoggingUserActionableMessage;
        }
    }
}
