// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Serialization;

namespace Pulumi.Automation.Events
{
    internal class EventLogWatcher : IAsyncDisposable
    {
        private readonly LocalSerializer _localSerializer = new LocalSerializer();
        private readonly CancellationTokenSource _internalCancellationTokenSource = new CancellationTokenSource();
        private readonly CancellationToken _externalCancellationToken;
        private readonly Action<EngineEvent> _onEvent;
        private readonly Task _pollingTask;

        private const int _pollingIntervalMilliseconds = 100;

        // We keep track of the length we have already read from the file
        private long _previousLength = 0;

        public string LogFile { get; }

        internal EventLogWatcher(
            string logFile
            , Action<EngineEvent> onEvent
            , CancellationToken externalCancellationToken)
        {
            LogFile = logFile;
            _onEvent = onEvent;
            _externalCancellationToken = externalCancellationToken;
            _pollingTask = Task.Run(PollForEvents);
        }

        public async ValueTask DisposeAsync()
        {
            _internalCancellationTokenSource.Cancel();
            await _pollingTask.ConfigureAwait(false);
        }

        private async Task PollForEvents()
        {
            var internalToken = _internalCancellationTokenSource.Token;
            var externalToken = _externalCancellationToken;

            using var linkedSource = CancellationTokenSource.CreateLinkedTokenSource(internalToken, externalToken);

            var linkedToken = linkedSource.Token;

            while (!linkedToken.IsCancellationRequested)
            {
                try
                {
                    // NOTE: Waiting before reading so that if ReadEvents throws we will
                    // wait before attempting it again
                    await Task.Delay(_pollingIntervalMilliseconds, linkedToken).ConfigureAwait(false);
                    ReadEvents();
                }
                catch (TaskCanceledException)
                {
                    break;
                }
                catch
                {
                    // We do our best effort to keep reading until the polling is cancelled
                }
            }

            // Try reading events one final time in case new events were written
            // while the task was cancelled during Task.Delay
            try
            {
                ReadEvents();
            }
            catch
            {
                // ignored
            }
        }

        private void ReadEvents()
        {
            if (!File.Exists(LogFile))
            {
                return;
            }

            using var fs = new FileStream(LogFile, FileMode.Open, FileAccess.Read);
            var newLength = fs.Length;

            if (newLength == _previousLength)
            {
                return;
            }

            fs.Seek(_previousLength, SeekOrigin.Begin);

            using var reader = new StreamReader(fs);

            var lines = reader
                .ReadToEnd()
                .Split(Environment.NewLine.ToCharArray(), StringSplitOptions.RemoveEmptyEntries);

            foreach (var line in lines)
            {
                var @event = _localSerializer.DeserializeJson<EngineEvent>(line);

                try
                {
                    _onEvent.Invoke(@event);
                }
                catch
                {
                    // Don't let the provided event handler cause reading of events to fail
                }
            }

            _previousLength = newLength;
        }
    }
}
