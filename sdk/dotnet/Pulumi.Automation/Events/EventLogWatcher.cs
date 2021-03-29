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
        private readonly CancellationTokenSource _internalCancellationTokenSource = new CancellationTokenSource();
        private readonly Action<EngineEvent> _onEvent;
        private const int _pollingIntervalMilliseconds = 100;

        // We keep track of the last position in the file.
        private long _position = 0;
        private bool _disposedValue;
        public string LogFile { get; }
        private Task _pollingTask;

        internal EventLogWatcher(
            string logFile
            , Action<EngineEvent> onEvent
            , CancellationToken externalCancellationToken)
        {
            LogFile = logFile;
            _onEvent = onEvent;
            _pollingTask = ReadEventsRepeatedly(externalCancellationToken);
            _pollingTask.Start();
        }

        /// Stops the polling loop and awaits the background task. Any exceptions encountered in the background 
        //  task will be propagated to the caller of this method.
        internal async Task Stop()
        {

            this._internalCancellationTokenSource.Cancel();
            await this._pollingTask;
        }

        private async Task ReadEventsRepeatedly(CancellationToken externalToken)
        {
            using var linkedSource = CancellationTokenSource.CreateLinkedTokenSource(
                this._internalCancellationTokenSource.Token,
                externalToken);
            var cancellationToken = linkedSource.Token;
            while (true)
            {
                await ReadEventsOnce(cancellationToken);
                await Task.Delay(_pollingIntervalMilliseconds, cancellationToken);
            }
        }

        private async Task ReadEventsOnce(CancellationToken cancellationToken)
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
                    cancellationToken.ThrowIfCancellationRequested();
                    _onEvent.Invoke(@event);
                }
            }
        }

        protected virtual void Dispose(bool disposing)
        {
            if (!_disposedValue)
            {
                if (disposing)
                {
                    _internalCancellationTokenSource.Dispose();
                }
                _disposedValue = true;
            }
        }

        public void Dispose()
        {
            // Do not change this code. Put cleanup code in 'Dispose(bool disposing)' method
            Dispose(disposing: true);
            GC.SuppressFinalize(this);
        }
    }
}
