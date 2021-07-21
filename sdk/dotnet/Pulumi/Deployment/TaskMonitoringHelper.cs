// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Linq;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Monitors dynamically added tasks for completion. Enters IDLE
    /// state when all monitored tasks finish. Allows awaiting next
    /// IDLE state or an exception, whichever comes first.
    /// Thread-safe.
    /// </summary>
    internal sealed class TaskMonitoringHelper
    {
        private readonly TaskExceptionTracker _exceptionTracker = new TaskExceptionTracker();
        private readonly TaskIdleTracker _idleTracker = new TaskIdleTracker();

        /// <summary>
        /// Starts monitoring the given task.
        /// </summary>
        public void AddTask(Task task)
        {
            _exceptionTracker.AddTask(task);
            _idleTracker.AddTask(task);
        }

        /// <summary>
        /// Awaits next IDLE state or an exception, whichever comes
        /// first. Several exceptions may be returned if they have
        /// been observed prior to this call.
        ///
        /// IDLE state is represented as an empty sequence in the result.
        /// </summary>
        public async Task<IEnumerable<Exception>> AwaitIdleOrFirstExceptionAsync()
        {
            var exceptionTask = _exceptionTracker.AwaitExceptionAsync();
            var idleTask = _idleTracker.AwaitIdleAsync();
            var firstTask = await Task.WhenAny((Task)exceptionTask, idleTask).ConfigureAwait(false);
            if (firstTask == idleTask)
            {
                return Enumerable.Empty<Exception>();
            }
            return await exceptionTask;
        }
    }

    /// <summary>
    /// Monitors dynamically added tasks for completion, allows awaiting IDLE state.
    /// </summary>
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

        /// <summary>
        /// Awaits next IDLE state when no monitored tasks are running.
        /// </summary>
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

        /// <summary>
        /// Monitors the given task.
        /// </summary>
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

    /// <summary>
    /// Monitors dynamically added tasks for exceptions, allows awaiting exceptions.
    /// </summary>
    internal sealed class TaskExceptionTracker
    {
        private readonly object _lockObject = new object();
        private readonly List<Exception> _exceptions = new List<Exception>();
        private TaskCompletionSource<IEnumerable<Exception>>? _promise;

        // Caches the delegate instance to avoid repeated allocations.
        private readonly Action<Task> _onTaskCompleted;

        public TaskExceptionTracker()
        {
            _onTaskCompleted = OnTaskCompleted;
        }

        /// <summary>
        /// Monitors the given task.
        /// </summary>
        public void AddTask(Task task)
        {
            task.ContinueWith(_onTaskCompleted);
        }

        /// <summary>
        /// Awaits the next set of `Exception` in the monitored tasks.
        /// May never complete. Never returns an empty sequence.
        /// </summary>
        public Task<IEnumerable<Exception>> AwaitExceptionAsync()
        {
            lock (_lockObject)
            {
                if (_exceptions.Count > 0)
                {
                    var err = Flush();
                    if (err != null)
                    {
                        return Task.FromResult(err);
                    }
                }
                if (_promise == null)
                {
                    _promise = new TaskCompletionSource<IEnumerable<Exception>>();
                }
                return _promise.Task;
            }
        }

        private IEnumerable<Exception> Flush()
        {
            // It is possible for multiple tasks to complete with the
            // same exception. This is happening in the test suite. It
            // is also possible to register the same task twice,
            // causing duplication.
            //
            // The `Distinct` here ensures this class does not report
            // the same exception twice to the single call of
            // `AwaitExceptionsAsync`.
            //
            // Note it is still possible to observe the same
            // exception twice from separate calls to
            // `AwaitExceptionsAsync`. This class opts not to keep
            // state to track that global invariant.
            var errs = _exceptions.Distinct().ToImmutableArray();
            _exceptions.Clear();
            return errs;
        }

        private void OnTaskCompleted(Task task)
        {
            if (!task.IsFaulted)
            {
                return;
            }
            AggregateException? errs = task.Exception;
            if (errs == null)
            {
                return;
            }
            lock (_lockObject)
            {
                _exceptions.AddRange(errs.InnerExceptions);
                if (_promise != null)
                {
                    var err = Flush();
                    if (err != null)
                    {
                        _promise.SetResult(err);
                    }
                    _promise = null;
                }
            }
        }
    }
}
