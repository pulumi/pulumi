using System;
using System.Threading.Tasks;

namespace Pulumi.Tests
{
    internal class TestDeploymentLogger : IEngineLogger
    {
        private Action<LogEntry> Logger { get; set; }

        public TestDeploymentLogger()
        {
            Logger = (entry) =>
            {
                Console.WriteLine($"IEngineLogger: {entry}");
            };
        }

        public TestDeploymentLogger(Action<LogEntry> logger)
        {
            Logger = logger;
        }

        public bool LoggedErrors { get; private set; }

        public Task SendAsync(string severity,
                              string message,
                              Resource? resource = null,
                              int? streamId = null,
                              bool? ephemeral = null)
        {
            if (severity == "Error")
            {
                LoggedErrors = true;
            }
            Logger.Invoke(new LogEntry
            {
                Severity = severity,
                Message = message,
                Resource = resource,
                StreamId = streamId,
                Ephemeral = ephemeral
            });
            return Task.FromResult(0);
        }

        public Task DebugAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => SendAsync("Debug", message, resource, streamId, ephemeral);

        public Task InfoAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => SendAsync("Info", message, resource, streamId, ephemeral);

        public Task WarnAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => SendAsync("Warn", message, resource, streamId, ephemeral);

        public Task ErrorAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null)
            => SendAsync("Error", message, resource, streamId, ephemeral);

        internal class LogEntry
        {
            public string Severity { get; set; } = "Error";
            public string Message { get; set; } = "";
            public Resource? Resource { get; set; }
            public int? StreamId { get; set; }
            public bool? Ephemeral { get; set; }

            public override string ToString()
            {
                return $"[{Severity}] {Message} [Resource={Resource} StreamId={StreamId} Ephemeral={Ephemeral}";
            }
        }
    }
}
