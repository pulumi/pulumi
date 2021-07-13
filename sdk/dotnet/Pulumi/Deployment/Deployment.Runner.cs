// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace Pulumi
{
    public partial class Deployment
    {
        private class Runner : IRunner
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
            private readonly Dictionary<Task, List<string>> _inFlightTasks = new Dictionary<Task, List<string>>();
            private readonly List<Exception> _exceptions = new List<Exception>();

            public ImmutableList<Exception> SwallowedExceptions => this._exceptions.ToImmutableList();

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

                lock (_inFlightTasks)
                {
                    // We may get several of the same tasks with different descriptions.  That can
                    // happen when the runtime reuses cached tasks that it knows are value-identical
                    // (for example Task.CompletedTask).  In that case, we just store all the
                    // descriptions. We'll print them all out as done once this task actually
                    // finishes.
                    if (!_inFlightTasks.TryGetValue(task, out var descriptions))
                    {
                        descriptions = new List<string>();
                        _inFlightTasks.Add(task, descriptions);
                    }

                    descriptions.Add(description);
                }
            }

            // Keep track if we already logged the information about an unhandled error to the user.  If
            // so, we end with a different exit code.  The language host recognizes this and will not print
            // any further messages to the user since we already took care of it.
            //
            // 32 was picked so as to be very unlikely to collide with any other error codes.
            private const int _processExitedAfterLoggingUserActionableMessage = 32;

            private async Task<int> WhileRunningAsync()
            {
                var tasks = new List<Task>();

                // Keep looping as long as there are outstanding tasks that are still running.
                while (true)
                {
                    tasks.Clear();
                    lock (_inFlightTasks)
                    {
                        if (_inFlightTasks.Count == 0)
                        {
                            // No more tasks in flight: exit the loop.
                            break;
                        }

                        // Grab all the tasks we currently have running.
                        tasks.AddRange(_inFlightTasks.Keys);
                    }

                    // Wait for one of the two events to happen:
                    // 1. All tasks in the list complete successfully, or
                    // 2. Any task throws an exception.
                    // There's no standard API with this semantics, so we create a custom completion source that is
                    // completed when remaining count is zero, or when an exception is thrown.
                    var remaining = tasks.Count;
                    var tcs = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
                    tasks.ForEach(HandleCompletion);
                    async void HandleCompletion(Task task)
                    {
                        try
                        {
                            // Wait for the task completion.
                            await task.ConfigureAwait(false);

                            // Log the descriptions of completed tasks.
                            List<string> descriptions;
                            lock (_inFlightTasks)
                            {
                                descriptions = _inFlightTasks[task];
                            }
                            foreach (var description in descriptions)
                            {
                                _deploymentLogger.LogDebug($"Completed task: {description}");
                            }

                            // Check if all the tasks are completed and signal the completion source if so.
                            if (Interlocked.Decrement(ref remaining) == 0)
                            {
                                tcs.TrySetResult(0);
                            }
                        }
                        catch (OperationCanceledException)
                        {
                            tcs.TrySetCanceled();
                        }
                        catch (Exception ex)
                        {
                            tcs.TrySetException(ex);
                        }
                        finally
                        {
                            // Once finished, remove the task from the set of tasks that are running.
                            lock (_inFlightTasks)
                            {
                                _inFlightTasks.Remove(task);
                            }
                        }
                    }
                    
                    try
                    {
                        // Now actually await that combined task and realize any exceptions it may have thrown.
                        await tcs.Task.ConfigureAwait(false);
                    }
                    catch (Exception e)
                    {
                        // if it threw, report it as necessary, then quit.
                        return await HandleExceptionAsync(e).ConfigureAwait(false);
                    }
                }

                // there were no more tasks we were waiting on.  Quit out, reporting if we had any
                // errors or not.
                return _deployment.Logger.LoggedErrors ? 1 : 0;
            }

            private async Task<int> HandleExceptionAsync(Exception exception)
            {
                this._exceptions.Add(exception);

                if (exception is LogException)
                {
                    // We got an error while logging itself.  Nothing to do here but print some errors
                    // and fail entirely.
                    _deploymentLogger.LogError(exception, "Error occurred trying to send logging message to engine");
                    await Console.Error.WriteLineAsync($"Error occurred trying to send logging message to engine:\n{exception.ToStringDemystified()}").ConfigureAwait(false);
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
                return _processExitedAfterLoggingUserActionableMessage;
            }
        }
    }
}
