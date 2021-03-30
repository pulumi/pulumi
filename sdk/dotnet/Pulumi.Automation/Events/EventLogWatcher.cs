// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Serialization;

namespace Pulumi.Automation.Events
{
    internal class EventLogWatcher : IDisposable
    {
        private readonly LocalSerializer _localSerializer = new LocalSerializer();
        private readonly Action<EngineEvent> _onEvent;
        private const int _pollingIntervalMilliseconds = 100;

        // We keep track of the last position in the file.
        private long _position = 0;
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

            await ReadEventsOnce();

            // At least one non-interruptible delay to ensure the thread has a chance
            // to read just-written data.
            await Task.Delay(_pollingIntervalMilliseconds); 

            while (true)
            {
                await ReadEventsOnce();
                await Task.Delay(_pollingIntervalMilliseconds, linkedSource.Token);
            }
        }

        private async Task ReadEventsOnce()
        {
            using var fs = new FileStream(LogFile, FileMode.Open, FileAccess.Read)
            {
                Position = this._position
            };
            using var reader = new StreamReader(fs);
            string? line;
            while (reader.Peek() >= 0)
            {
                line = await reader.ReadLineAsync();
                this._position = fs.Position;
                if (!String.IsNullOrWhiteSpace(line))
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
