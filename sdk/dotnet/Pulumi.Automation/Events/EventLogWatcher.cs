// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Serialization;

namespace Pulumi.Automation.Events
{
    internal sealed class EventLogWatcher : IDisposable
    {
        private readonly LocalSerializer _localSerializer = new LocalSerializer();
        private readonly Action<EngineEvent> _onEvent;
        private const int _pollingIntervalMilliseconds = 100;

        // We keep track of the last position in the file.
        private long _position;
        public string LogFile { get; }
        private readonly Task _pollingTask;
        private readonly CancellationTokenSource _internalCancellationTokenSource = new CancellationTokenSource();

        private CancellationToken? _cancellationToken;

        internal EventLogWatcher(
            string logFile
            , Action<EngineEvent> onEvent
            , CancellationToken externalCancellationToken)
        {
            LogFile = logFile;
            _onEvent = onEvent;
            _pollingTask = PollForEvents(externalCancellationToken);
        }

        /// Stops the polling loop and awaits the background task. Any exceptions encountered in the background
        /// task will be propagated to the caller of this method.
        internal async Task Stop()
        {
            this._internalCancellationTokenSource.Cancel();
            await this.AwaitPollingTask();

            // Race condition workaround.
            //
            // The caller might consider Pulumi CLI sub-process
            // finished and its writes committed to the file system.
            // However we do not truly know if the reader thread has
            // had a chance to consume them yet.
            //
            // To work around we do one more non-interruptible delay
            // and a final read pass here.
            //
            // A proper solution would involve having Pulumi CLI emit
            // a CommandDone or some such EngineEvent, and we would
            // keep reading until we see one.

            await Task.Delay(_pollingIntervalMilliseconds);
            await ReadEventsOnce();
        }

        /// Exposed for testing; use Stop instead.
        internal async Task AwaitPollingTask()
        {
            try
            {
                await this._pollingTask;
            }
            catch (OperationCanceledException error) when (error.CancellationToken == this._cancellationToken)
            {
                // _pollingTask.State == Cancelled
            }
        }

        private async Task PollForEvents(CancellationToken externalCancellationToken)
        {
            using var linkedSource = CancellationTokenSource.CreateLinkedTokenSource(
                this._internalCancellationTokenSource.Token,
                externalCancellationToken);
            this._cancellationToken = linkedSource.Token;

            while (true)
            {
                await ReadEventsOnce();
                await Task.Delay(_pollingIntervalMilliseconds, linkedSource.Token);
            }
            // ReSharper disable once FunctionNeverReturns
        }

        private async Task ReadEventsOnce()
        {
            if (!File.Exists(LogFile))
            {
                return;
            }

            await using var fs = new FileStream(LogFile, FileMode.Open, FileAccess.Read, FileShare.ReadWrite) { Position = this._position };
            using var reader = new StreamReader(fs);
            while (reader.Peek() >= 0)
            {
                var line = await reader.ReadLineAsync();
                this._position = fs.Position;
                if (!string.IsNullOrWhiteSpace(line))
                {
                    line = line.Trim();
                    var @event = _localSerializer.DeserializeJson<EngineEvent>(line);
                    _onEvent.Invoke(@event);
                }
            }
        }

        public void Dispose()
        {
            _internalCancellationTokenSource.Dispose();
        }
    }
}
