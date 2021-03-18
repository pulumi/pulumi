
using System.Collections.Generic;
using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class EngineEventModel : IJsonModel<EngineEvent>
    {
        public int Sequence { get; set; }

        public int Timestamp { get; set; }

        public SummaryEventModel? SummaryEvent { get; set; }

        public DiagnosticEventModel? DiagnosticEvent { get; set; }

        public CancelEventModel? CancelEvent { get; set; }

        public StdoutEngineEventModel? StdoutEvent { get; set; }

        // TODO: Rest of the events

        public EngineEvent Convert() =>
            new EngineEvent(
                Sequence,
                Timestamp,
                SummaryEvent?.Convert(),
                DiagnosticEvent?.Convert(),
                CancelEvent?.Convert(),
                StdoutEvent?.Convert());
    }

    internal class CancelEventModel : IJsonModel<CancelEvent>
    {
        public CancelEvent Convert() =>
            new CancelEvent();
    }

    internal class SummaryEventModel : IJsonModel<SummaryEvent>
    {
        public bool MaybeCorrupt { get; set; }

        public int DurationSeconds { get; set; }

        public Dictionary<OperationType, int> ResourceChanges { get; set; } = null!;

        public Dictionary<string, string> PolicyPacks { get; set; } = null!;

        public SummaryEvent Convert() =>
            new SummaryEvent(
                MaybeCorrupt,
                DurationSeconds,
                ResourceChanges ?? new Dictionary<OperationType, int>(),
                PolicyPacks ?? new Dictionary<string, string>());
    }

    internal class StdoutEngineEventModel : IJsonModel<StdoutEngineEvent>
    {
        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public StdoutEngineEvent Convert() =>
            new StdoutEngineEvent(Message, Color);
    }

    internal class DiagnosticEventModel : IJsonModel<DiagnosticEvent>
    {
        public string? Urn { get; set; }

        public string? Prefix { get; set; }

        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        // TODO: Make this into enum?
        public string Severity { get; set; } = null!;

        public string? StreamId { get; set; }

        public bool? Ephemeral { get; set; }

        public DiagnosticEvent Convert() =>
            new DiagnosticEvent(
                Urn,
                Prefix,
                Message,
                Color,
                Severity,
                StreamId,
                Ephemeral);
    }
}
