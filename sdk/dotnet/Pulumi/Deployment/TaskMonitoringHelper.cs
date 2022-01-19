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
        private readonly object _lockObject = new object();
        private int _activeTasks;

        private readonly List<Exception> _exceptions = new List<Exception>();

        private TaskCompletionSource<IEnumerable<Exception>>? _promise;

        // Caches the delegate instance to avoid repeated allocations.
        private readonly Action<Task> _onTaskCompleted;

        public TaskMonitoringHelper()
        {
            _onTaskCompleted = OnTaskCompleted;
        }

        /// <summary>
        /// Starts monitoring the given task.
        /// </summary>
        public void AddTask(Task task)
        {
            lock (_lockObject)
            {
                _activeTasks++;
            }
            task.ContinueWith(_onTaskCompleted);
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
            lock (_lockObject)
            {
                _activeTasks--;

                if (task.IsFaulted && task.Exception != null)
                {
                    _exceptions.AddRange(task.Exception.InnerExceptions);
                }

                if (_exceptions.Count > 0 && _promise != null)
                {
                    _promise.SetResult(Flush());
                    _promise = null;
                }
                else if (_activeTasks == 0 && _promise != null)
                {
                    _promise.SetResult(Enumerable.Empty<Exception>());
                    _promise = null;
                }
            }
        }

        /// <summary>
        /// Awaits next IDLE state or an exception, whichever comes
        /// first. Several exceptions may be returned if they have
        /// been observed prior to this call.
        ///
        /// IDLE state is represented as an empty sequence in the result.
        /// </summary>
        public Task<IEnumerable<Exception>> AwaitIdleOrFirstExceptionAsync()
        {
            lock (_lockObject)
            {
                if (_exceptions.Count > 0)
                {
                    return Task.FromResult(Flush());
                }
                else if (_activeTasks == 0)
                {
                    return Task.FromResult(Enumerable.Empty<Exception>());
                }
                else
                {
                    if (_promise == null)
                    {
                        _promise = new TaskCompletionSource<IEnumerable<Exception>>();
                    }
                    return _promise.Task;
                }
            }
        }
    }
}
