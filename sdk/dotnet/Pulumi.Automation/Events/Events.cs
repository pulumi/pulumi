// Copyright 2016-2021, Pulumi Corporation
using System.Collections.Generic;
using System.Collections.Immutable;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Events
{
    // TODO(vipentti): internal -> public, and add to Unshipped.txt once ready
    // TODO(vipentti): Split to separate files?
    internal class EngineEvent
    {
        public int Sequence { get; }

        public int Timestamp { get; }

        public SummaryEvent? SummaryEvent { get; }

        public DiagnosticEvent? DiagnosticEvent { get; }

        public CancelEvent? CancelEvent { get; }

        public StdoutEngineEvent? StdoutEngineEvent { get; }

        internal EngineEvent(
            int sequence,
            int timestamp,
            SummaryEvent? summaryEvent,
            DiagnosticEvent? diagnosticEvent,
            CancelEvent? cancelEvent,
            StdoutEngineEvent? stdoutEngineEvent)
        {
            Sequence = sequence;
            Timestamp = timestamp;
            SummaryEvent = summaryEvent;
            DiagnosticEvent = diagnosticEvent;
            CancelEvent = cancelEvent;
            StdoutEngineEvent = stdoutEngineEvent;
        }
    }

    internal class CancelEvent
    {
        internal CancelEvent() { }
    }

    internal class SummaryEvent
    {
        public bool MaybeCorrupt { get; }

        public int DurationSeconds { get; }

        public IImmutableDictionary<OperationType, int> ResourceChanges { get; }

        public IImmutableDictionary<string, string> PolicyPacks { get; }

        internal SummaryEvent(
            bool maybeCorrupt,
            int durationSeconds,
            IDictionary<OperationType, int> resourceChanges,
            IDictionary<string, string> policyPacks)
        {
            MaybeCorrupt = maybeCorrupt;
            DurationSeconds = durationSeconds;
            ResourceChanges = resourceChanges.ToImmutableDictionary();
            PolicyPacks = policyPacks.ToImmutableDictionary();
        }
    }

    internal class StdoutEngineEvent
    {
        public string Message { get; }

        public string Color { get; }

        internal StdoutEngineEvent(string message, string color)
        {
            Message = message;
            Color = color;
        }
    }

    internal class DiagnosticEvent
    {
        public string? Urn { get; }

        public string? Prefix { get; }

        public string Message { get; }

        public string Color { get; }

        // TODO: Make this into enum?
        public string Severity { get; }

        public string? StreamId { get; }

        public bool? Ephemeral { get; }

        internal  DiagnosticEvent(
            string? urn,
            string? prefix,
            string message,
            string color,
            string severity,
            string? streamId,
            bool? ephemeral)
        {
            Urn = urn;
            Prefix = prefix;
            Message = message;
            Color = color;

            Severity = severity;
            StreamId = streamId;
            Ephemeral = ephemeral;
        }
    }
}
