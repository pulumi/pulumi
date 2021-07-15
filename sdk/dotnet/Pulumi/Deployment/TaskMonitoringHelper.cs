// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi
{
    /// Monitors dynamically added tasks for completion. Enters IDLE
    /// state when all monitored tasks finish. Allows awaiting next
    /// IDLE state or the first exception, whichever comes first.
    /// Thread-safe.
    internal sealed class TaskMonitoringHelper
    {
        private readonly object _lockObject = new object();
        private int _activeTasks = 0;
        private TaskCompletionSource<Exception?>? _next = null;

        public TaskMonitoringHelper()
        {
            CheckInvariants();
        }

        /// Adds the task to the task-set being monitored.
        public void AddTask(Task task)
        {
            lock (_lockObject)
            {
                // Check if we are moving out of IDLE state.
                if (_activeTasks == 0)
                {
                    // Fresh promise is needed.
                    _next = new TaskCompletionSource<Exception?>();
                }
                _activeTasks++;
                CheckInvariants();
            };
            task.ContinueWith(this.OnTaskCompleted);
        }

        /// Awaits the next IDLE period (reprsented by null) or the first exception
        /// encountered in the monitored tasks.
        public Task<Exception?> AwaitIdleOrFirstExceptionAsync()
        {
            lock (_lockObject)
            {
                if (_next != null)
                {
                    return _next.Task;
                } 
                else
                {
                    // In IDLE state already.
                    return Task.FromResult<Exception?>(null);
                }
            }
        }

        private void OnTaskCompleted(Task task)
        {
            lock (_lockObject)
            {
                _activeTasks--;

                // If entering IDLE state or observing an exception, 
                // notify waiters and reset state with a fresh promise.
                if (_activeTasks == 0 || task.Exception != null)
                {
                    if (_next != null)
                    {
                        _next.SetResult(task.Exception);
                    }
                    _next = new TaskCompletionSource<Exception?>();
                }

                // Clean up if entering IDLE state.
                if (_activeTasks == 0)
                {
                    _next = null;
                }

                CheckInvariants();
            }
        }

        private void CheckInvariants()
        {
            if (_activeTasks == 0 && _next == null)
            {
                // idle state
                return;
            }
            if (_activeTasks > 0 && _next != null)
            {
                // active state
                return;
            }
            throw new Exception("TaskMonitoringHelper: instance state invariants violated");
        }
    }
}
