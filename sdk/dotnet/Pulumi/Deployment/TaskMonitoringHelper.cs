// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Linq;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi
{
    /// Monitors dynamically added tasks for completion. Enters IDLE
    /// state when all monitored tasks finish. Allows awaiting next
    /// IDLE state or an exception, whichever comes first.
    /// Thread-safe.
    internal sealed class TaskMonitoringHelper
    {
        private readonly TaskExceptionTracker errTracker = new TaskExceptionTracker();
        private readonly TaskIdleTracker idleTracker = new TaskIdleTracker();

        /// Starts monitoring the given task.
        public void AddTask(Task task)
        {
            errTracker.AddTask(task);
            idleTracker.AddTask(task);
        }

        /// Awaits next IDLE state or an exception, whichever comes first.
        /// IDLE state is represented as `null` in the result.
        public async Task<Exception?> AwaitIdleOrFirstExceptionAsync()
        {
            var error = errTracker.AwaitExceptionAsync();
            var idle = idleTracker.AwaitIdleAsync();
            var first = await Task.WhenAny((Task)error, idle);
            if (first == idle)
            {
                return null;
            }
            var err = await error;
            return err;
        }
    }

    /// Monitors dynamically added tasks for completion, allows awaiting IDLE state.
    internal sealed class TaskIdleTracker
    {
        private readonly object _lockObject = new object();
        private int _activeTasks;
        private TaskCompletionSource<int>? _promise;

        // Caches the delegate instance to avoid repeated allocations.
        private readonly Action<Task> _onTaskCompleted;

        public TaskIdleTracker()
        {
            _onTaskCompleted = OnTaskCompleted;
        }

        /// Awaits next IDLE state when no monitored tasks are running.
        public Task AwaitIdleAsync()
        {
            lock (_lockObject)
            {
                if (_activeTasks == 0)
                {
                    return Task.FromResult(0);
                }
                if (_promise == null)
                {
                    _promise = new TaskCompletionSource<int>();
                }
                return _promise.Task;
            }
        }

        /// Monitors the given task.
        public void AddTask(Task task)
        {
            lock (_lockObject)
            {
                _activeTasks++;
            }
            task.ContinueWith(_onTaskCompleted);
        }

        private void OnTaskCompleted(Task task)
        {
            lock (_lockObject)
            {
                _activeTasks--;
                if (_activeTasks == 0 && _promise != null)
                {
                    _promise.SetResult(0);
                    _promise = null;
                }
            }
        }
    }

    /// Monitors dynamically added tasks for exceptions, allows awaiting exceptions.
    internal sealed class TaskExceptionTracker
    {
        private readonly object _lockObject = new object();
        private readonly List<Exception> _exceptions = new List<Exception>();
        private TaskCompletionSource<Exception>? _promise;

        // Caches the delegate instance to avoid repeated allocations.
        private readonly Action<Task> _onTaskCompleted;

        public TaskExceptionTracker()
        {
            _onTaskCompleted = OnTaskCompleted;
        }

        /// Monitors the given task.
        public void AddTask(Task task)
        {
            task.ContinueWith(_onTaskCompleted);
        }

        /// Awaits the next `Exception` in the monitored tasks. May never complete. May return
        /// `AggregateException` if more than one monitored task fails.
        public Task<Exception> AwaitExceptionAsync()
        {
            lock (_lockObject)
            {
                if (_exceptions.Count > 0)
                {
                    return Task.FromResult(Flush());
                }
                if (_promise == null)
                {
                    _promise = new TaskCompletionSource<Exception>();
                }
                return _promise.Task;
            }
        }

        private Exception Flush()
        {
            var err = CombineExceptions(_exceptions[0], _exceptions.Skip(1));
            _exceptions.Clear();
            return err;
        }

        private void OnTaskCompleted(Task task)
        {
            if (!task.IsFaulted)
            {
                return;
            }
            AggregateException? errs = task.Exception;
            if (errs != null)
            {
                lock (_lockObject)
                {
                    _exceptions.AddRange(errs.InnerExceptions);
                    if (_promise != null)
                    {
                        _promise.SetResult(Flush());
                        _promise = null;
                    }
                }
            }
        }

        /// Packs an AggregateException if necesssary.
        private static Exception CombineExceptions(Exception exception, IEnumerable<Exception> exceptions)
        {
            if (!exceptions.Any())
            {
                return exception;
            }
            else
            {
                return new AggregateException((new []{exception}).Concat(exceptions));
            }
        }
    }
}
