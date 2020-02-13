﻿// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi
{
    public partial class Deployment
    {
        private class Runner : IRunner
        {
            private readonly IDeploymentInternal _deployment;

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

            public Runner(IDeploymentInternal deployment)
                => _deployment = deployment;

            public Task<int> RunAsync<TStack>() where TStack : Stack, new()
            {
                try
                {
                    var stack = new TStack();
                    // Stack doesn't call RegisterOutputs, so we register them on its behalf.
                    stack.RegisterPropertyOutputs();
                    RegisterTask("User program code.", stack.Outputs.DataTask);
                }
                catch (Exception ex)
                {
                    return HandleExceptionAsync(ex);
                }

                return WhileRunningAsync();
            }

            public Task<int> RunAsync(Func<Task<IDictionary<string, object?>>> func)
            {
                var stack = new Stack(func);
                RegisterTask("User program code.", stack.Outputs.DataTask);
                return WhileRunningAsync();
            }

            public void RegisterTask(string description, Task task)
            {
                Serilog.Log.Information($"Registering task: {description}");

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
                            break;
                        }

                        // Grab all the tasks we currently have running.
                        tasks.AddRange(_inFlightTasks.Keys);
                    }

                    // Now, wait for one of them to finish.
                    var task = await Task.WhenAny(tasks).ConfigureAwait(false);
                    List<string> descriptions;
                    lock (_inFlightTasks)
                    {
                        // Once finished, remove it from the set of tasks that are running.
                        descriptions = _inFlightTasks[task];
                        _inFlightTasks.Remove(task);
                    }

                    foreach (var description in descriptions)
                        Serilog.Log.Information($"Completed task: {description}");

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
                return _deployment.Logger.LoggedErrors ? 1 : 0;
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
                    await _deployment.Logger.ErrorAsync(exception.Message).ConfigureAwait(false);
                }
                else if (exception is ResourceException resourceEx)
                {
                    var message = resourceEx.HideStack
                        ? resourceEx.Message
                        : resourceEx.ToString();
                    await _deployment.Logger.ErrorAsync(message, resourceEx.Resource).ConfigureAwait(false);
                }
                else
                {
                    var location = System.Reflection.Assembly.GetEntryAssembly()?.Location;
                    await _deployment.Logger.ErrorAsync(
    $@"Running program '{location}' failed with an unhandled exception:
{exception.ToString()}").ConfigureAwait(false);
                }

                Serilog.Log.Debug("Wrote last error.  Returning from program.");
                return _processExitedAfterLoggingUserActionableMessage;
            }
        }
    }
}
