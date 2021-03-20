// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using Pulumi.Automation.Serialization;

namespace Pulumi.Automation.Events
{
    internal class EventLogWatcher : IDisposable
    {
        // Used to ensure only one event handler ends up reading the file at a time
        private readonly object _eventLogReaderLock = new object();
        private readonly LocalSerializer _localSerializer = new LocalSerializer();
        private readonly FileSystemWatcher _fileSystemWatcher;
        private readonly Action<EngineEvent> _onEvent;

        // We keep track of the length we have already read from the file
        private long _previousLength = 0;

        public string LogFile { get; }

        internal EventLogWatcher(
            string logFile
            , Action<EngineEvent> onEvent)
        {
            LogFile = logFile;

            _onEvent = onEvent;

            _fileSystemWatcher = new FileSystemWatcher
            {
                Path = Path.GetDirectoryName(LogFile),
                Filter = Path.GetFileName(LogFile),
                NotifyFilter = NotifyFilters.LastWrite | NotifyFilters.Size,
                EnableRaisingEvents = true,
            };
            _fileSystemWatcher.Changed += HandleChangedEvent;
        }

        public void Dispose()
        {
            _fileSystemWatcher.Dispose();
        }

        private void HandleChangedEvent(object sender, FileSystemEventArgs args)
        {
            lock(_eventLogReaderLock)
            {
                using var fs = new FileStream(args.FullPath, FileMode.Open, FileAccess.Read);
                var newLength = fs.Length;
                fs.Seek(_previousLength, SeekOrigin.Begin);

                using var reader = new StreamReader(fs);

                var lines = reader
                    .ReadToEnd()
                    .Split(Environment.NewLine.ToCharArray(), StringSplitOptions.RemoveEmptyEntries);

                foreach (var line in lines)
                {
                    var @event = _localSerializer.DeserializeJson<EngineEvent>(line);

                    _onEvent.Invoke(@event);
                }

                _previousLength = newLength;
            }
        }
    }
}
