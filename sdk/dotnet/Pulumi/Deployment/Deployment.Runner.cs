// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace Pulumi
{
    public partial class Deployment
    {
        internal class Runner : IRunner
        {
            private readonly IDeploymentInternal _deployment;
            private readonly ILogger _deploymentLogger;

            /// <summary>
            /// The set of tasks that we have fired off.  We issue tasks in a Fire-and-Forget manner
            /// to be able to expose a Synchronous <see cref="Resource"/> model for users. i.e. a
            /// user just synchronously creates a resource, and we asynchronously kick off the work
            /// to populate it.  This works well, however we have to make sure the console app
            /// doesn't exit because it thinks there is no work to do.
            /// <para/>
            /// To ensure that doesn't happen, we have the main entrypoint of the app just
            /// continuously, asynchronously loop, waiting for these tasks to complete, and only
            /// exiting once the set becomes empty.
            /// </summary>
            private readonly TaskMonitoringHelper _inFlightTasks = new TaskMonitoringHelper();

            private readonly object _exceptionsLock = new object();
            private readonly List<Exception> _exceptions = new List<Exception>();

            private readonly ConcurrentDictionary<(int TaskId, string Desc),int> _descriptions =
                new ConcurrentDictionary<(int TaskId, string Desc),int>();

            public ImmutableList<Exception> SwallowedExceptions
            {
                get
                {
                    lock (_exceptionsLock)
                    {
                        return _exceptions.ToImmutableList();
                    }
                }
            }

            public Runner(IDeploymentInternal deployment, ILogger deploymentLogger)
            {
                _deployment = deployment;
                _deploymentLogger = deploymentLogger;
            }

            Task<int> IRunner.RunAsync<TStack>(IServiceProvider serviceProvider)
            {
                if (serviceProvider == null)
                {
                    throw new ArgumentNullException(nameof(serviceProvider));
                }

                return RunAsync(() => serviceProvider.GetService(typeof(TStack)) as TStack
                    ?? throw new ApplicationException($"Failed to resolve instance of type {typeof(TStack)} from service provider. Register the type with the service provider before calling {nameof(RunAsync)}."));
            }

            Task<int> IRunner.RunAsync<TStack>() => RunAsync(() => new TStack());

            public Task<int> RunAsync<TStack>(Func<TStack> stackFactory) where TStack : Stack
            {
                try
                {
                    var stack = stackFactory();
                    // Stack doesn't call RegisterOutputs, so we register them on its behalf.
                    stack.RegisterPropertyOutputs();
                    RegisterTask($"{nameof(RunAsync)}: {stack.GetType().FullName}", stack.Outputs.DataTask);
                }
                catch (Exception ex)
                {
                    return HandleExceptionAsync(ex);
                }

                return WhileRunningAsync();
            }

            Task<int> IRunner.RunAsync(Func<Task<IDictionary<string, object?>>> func, StackOptions? options)
            {
                var stack = new Stack(func, options);
                RegisterTask($"{nameof(RunAsync)}: {stack.GetType().FullName}", stack.Outputs.DataTask);
                return WhileRunningAsync();
            }

            public void RegisterTask(string description, Task task)
            {
                _deploymentLogger.LogDebug($"Registering task: {description}");
                _inFlightTasks.AddTask(task);

                // Ensure completion message is logged at most once when the task finishes.
                if (_deploymentLogger.IsEnabled(LogLevel.Debug))
                {
                    // We may get several of the same tasks with different descriptions.  That can
                    // happen when the runtime reuses cached tasks that it knows are value-identical
                    // (for example Task.CompletedTask).  In that case, we just store all the
                    // descriptions. We'll print them all out as done once this task actually
                    // finishes.

                    var key = (TaskId: task.Id, Desc: description);
                    int timesSeen = _descriptions.AddOrUpdate(key, _ => 1, (_, v) => v + 1);
                    if (timesSeen == 1)
                    {
                        task.ContinueWith(task => {
                            _deploymentLogger.LogDebug($"Completed task: {description}");
                            _descriptions.TryRemove(key, out _);
                        });
                    }
                }
            }

            // Keep track if we already logged the information about an unhandled error to the user.  If
            // so, we end with a different exit code.  The language host recognizes this and will not print
            // any further messages to the user since we already took care of it.
            //
            // 32 was picked so as to be very unlikely to collide with any other error codes.
            private const int _processExitedAfterLoggingUserActionableMessage = 32;

            internal async Task<int> WhileRunningAsync()
            {
                var errs = await _inFlightTasks.AwaitIdleOrFirstExceptionAsync().ConfigureAwait(false);
                if (errs.Any())
                {
                    return await HandleExceptionsAsync(errs).ConfigureAwait(false);
                }

                // there were no more tasks we were waiting on.  Quit out, reporting if we had any
                // errors or not.
                return _deployment.Logger.LoggedErrors ? 1 : 0;
            }

            private Task<int> HandleExceptionAsync(Exception exception)
            {
                return HandleExceptionsAsync(new Exception[]{exception});
            }

            private async Task<int> HandleExceptionsAsync(IEnumerable<Exception> exceptions)
            {
                if (!exceptions.Any())
                {
                    return 0;
                }
                lock (_exceptionsLock)
                {
                    _exceptions.AddRange(exceptions);
                }

                var loggedExceptionCount = 0;
                foreach (var exception in exceptions)
                {
                    var logged = await LogExceptionToErrorStream(exception);
                    loggedExceptionCount += logged ? 1 : 0;
                }

                // If we logged any exceptions, then return with a
                // special error code stating as such so that our host
                // does not print out another set of errors.
                var exitCode = (loggedExceptionCount > 0)
                    ? _processExitedAfterLoggingUserActionableMessage
                    : 1;

                // We set the exit code explicitly here in case users
                // do not bubble up the exit code themselves to
                // top-level entry point of the program. For example
                // when they `await Deployment.RunAsync()` instead of 
                // `return await Deployment.RunAsync()`
                Environment.ExitCode = exitCode;

                return exitCode;
            }

            private async Task<bool> LogExceptionToErrorStream(Exception exception)
            {
                if (exception is LogException)
                {
                    // We got an error while logging itself. Nothing
                    // to do here but print some errors and abort.
                    _deploymentLogger.LogError(exception, "Error occurred trying to send logging message to engine");
                    await Console.Error.WriteLineAsync($"Error occurred trying to send logging message to engine:\n{exception.ToStringDemystified()}").ConfigureAwait(false);
                    return false;
                }

                // For all other issues we encounter we log the
                // problem to the error stream.
                //
                // Note: if these logging calls fail, they will just
                // end up bubbling up an exception that will be caught
                // by nothing. This will tear down the actual process
                // with a non-zero error which our host will handle
                // properly.
                if (exception is RunException)
                {
                    // Always hide the stack for RunErrors.
                    await _deployment.Logger.ErrorAsync(exception.Message).ConfigureAwait(false);
                }
                else if (exception is ResourceException resourceEx)
                {
                    var message = resourceEx.HideStack ? resourceEx.Message : resourceEx.ToStringDemystified();
                    await _deployment.Logger.ErrorAsync(message, resourceEx.Resource).ConfigureAwait(false);
                }
                else
                {
                    var location = Assembly.GetEntryAssembly()?.Location;
                    await _deployment.Logger.ErrorAsync($"Running program '{location}' failed with an unhandled exception:\n{exception.ToStringDemystified()}").ConfigureAwait(false);
                }

                _deploymentLogger.LogDebug("Returning from program after last error");
                return true;
            }
        }
    }
}
