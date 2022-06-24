// Copyright 2016-2021, Pulumi Corporation

using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Pulumi.Automation.Events;
using Xunit;

namespace Pulumi.Automation.Tests
{

    public class EventLogWatcherTests
    {

        [Fact]
        public async Task ReceivesBasicEvent()
        {
            using var fx = new Fixture();
            await fx.Write("{}");
            await fx.Watcher.Stop();
            Assert.Equal(1, fx.EventCounter);
        }

        [Fact]
        public async Task ReceivesManyBasicEvents()
        {
            using var fx = new Fixture();
            for (var i = 0; i < 10; i++)
            {
                await fx.Write("{}");
            }
            await fx.Watcher.Stop();
            Assert.Equal(10, fx.EventCounter);
        }

        [Fact]
        public async Task PropagatesUserExceptionsToCaller()
        {
            using var fx = new Fixture { Action = ev => throw new MyException() };

            await fx.Write("{}");

            await Assert.ThrowsAsync<MyException>(async () =>
            {
                await fx.Watcher.Stop();
            });
        }

        [Fact]
        public async Task PermitsUserInitiatedCancellation()
        {
            using var fx = new Fixture();
            fx.CancellationTokenSource.Cancel();
            await fx.Watcher.AwaitPollingTask();
        }

        private class MyException : Exception { }

        private sealed class Fixture : IDisposable
        {
            public int EventCounter;
            public string LogFileName { get; }
            public StreamWriter Writer { get; }
            public EventLogWatcher Watcher { get; }
            public CancellationTokenSource CancellationTokenSource { get; }

            public Action<EngineEvent>? Action { get; set; }
            public Fixture()
            {
                this.CancellationTokenSource = new CancellationTokenSource();
                this.LogFileName = Path.GetTempFileName();
                this.Writer = new StreamWriter(this.LogFileName);
                this.Watcher = new EventLogWatcher(this.LogFileName, onEvent: ev =>
                {
                    Interlocked.Increment(ref this.EventCounter);
                    this.Action?.Invoke(ev);
                }, this.CancellationTokenSource.Token);
            }

            public async Task Write(string text)
            {
                await this.Writer.WriteLineAsync(text);
                await this.Writer.FlushAsync();
            }

            public void Dispose()
            {
                this.Watcher.Dispose();
                this.CancellationTokenSource.Dispose();
                this.Writer.Dispose();
                File.Delete(this.LogFileName);
            }
        }
    }
}
