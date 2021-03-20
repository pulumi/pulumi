// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Events
{
    // TODO(vipentti): internal -> public, and add to Unshipped.txt once ready
    // TODO(vipentti): Split to separate files?
    // TODO(vipentti): Add doc comments based on apitype/events.go

    internal class CancelEvent
    {
        internal CancelEvent() {}
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

        internal DiagnosticEvent(
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

    internal class PolicyEvent
    {
        public string? ResourceUrn { get; }

        public string Message { get; }

        public string Color { get; }

        public string PolicyName { get; }

        public string PolicyPackName { get; }

        public string PolicyPackVersion { get; }

        public string PolicyPackVersionTag { get; }

        public string EnforcementLevel { get; }

        internal PolicyEvent(
            string? resourceUrn,
            string message,
            string color,
            string policyName,
            string policyPackName,
            string policyPackVersion,
            string policyPackVersionTag,
            string enforcementLevel)
        {
            ResourceUrn = resourceUrn;
            Message = message;
            Color = color;
            PolicyName = policyName;
            PolicyPackName = policyPackName;
            PolicyPackVersion = policyPackVersion;
            PolicyPackVersionTag = policyPackVersionTag;
            EnforcementLevel = enforcementLevel;
        }
    }


    internal class PreludeEvent
    {
        public IImmutableDictionary<string, string> Config { get; }

        internal PreludeEvent(IDictionary<string, string> config)
        {
            Config = config.ToImmutableDictionary();
        }
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

    internal enum DiffKind
    {
        Add,
        AddReplace,
        Delete,
        DeleteReplace,
        Update,
        UpdateReplace,
    }

    internal class PropertyDiff
    {
        public DiffKind Kind { get; }

        public bool InputDiff { get; }

        internal PropertyDiff(DiffKind kind, bool inputDiff)
        {
            Kind = kind;
            InputDiff = inputDiff;
        }
    }

    internal class StepEventMetadata
    {
        public OperationType Op { get; }

        public string Urn { get; }

        public string Type { get; }

        public StepEventStateMetadata? Old { get; }

        // TODO: can this actually ever be null?
        public StepEventStateMetadata? New { get; }

        public ImmutableArray<string>? Keys { get; }

        public ImmutableArray<string>? Diffs { get; }

        public IImmutableDictionary<string, PropertyDiff>? DetailedDiff { get; }

        public bool? Logical { get; }

        public string Provider { get; }

        internal StepEventMetadata(
            OperationType op,
            string urn,
            string type,
            StepEventStateMetadata? old,
            StepEventStateMetadata? @new,
            IEnumerable<string>? keys,
            IEnumerable<string>? diffs,
            IDictionary<string, PropertyDiff>? detailedDiff,
            bool? logical,
            string provider)
        {
            Op = op;
            Urn = urn;
            Type = type;
            Old = old;
            New = @new;
            Keys = keys?.ToImmutableArray();
            Diffs = diffs?.ToImmutableArray();
            DetailedDiff = detailedDiff?.ToImmutableDictionary();
            Logical = logical;
            Provider = provider;
        }
    }

    internal class StepEventStateMetadata
    {
        public string Urn { get; }

        public string Type { get; }

        public bool? Custom { get; }

        public bool? Delete { get; }

        public string Id { get; }

        public string Parent { get; }

        public bool? Protect { get; }

        public IImmutableDictionary<string, object> Inputs { get; }

        public IImmutableDictionary<string, object> Outputs { get; }

        public string Provider { get; }

        public ImmutableArray<string>? InitErrors { get; }

        internal StepEventStateMetadata(
            string urn,
            string type,
            bool? custom,
            bool? delete,
            string id,
            string parent,
            bool? protect,
            IDictionary<string, object> inputs,
            IDictionary<string, object> outputs,
            string provider,
            IEnumerable<string>? initErrors)
        {
            Urn = urn;
            Type = type;
            Custom = custom;
            Delete = delete;
            Id = id;
            Parent = parent;
            Protect = protect;
            Inputs = inputs.ToImmutableDictionary();
            Outputs = outputs.ToImmutableDictionary();
            Provider = provider;
            InitErrors = initErrors?.ToImmutableArray();
        }
    }

    internal class ResourcePreEvent
    {
        public StepEventMetadata Metadata { get; }
        public bool? Planning { get; }

        internal ResourcePreEvent(StepEventMetadata metadata, bool? planning)
        {
            Metadata = metadata;
            Planning = planning;
        }
    }

    internal class ResOutputsEvent
    {
        public StepEventMetadata Metadata { get; }
        public bool? Planning { get; }

        internal ResOutputsEvent(StepEventMetadata metadata, bool? planning)
        {
            Metadata = metadata;
            Planning = planning;
        }
    }

    internal class ResOpFailedEvent
    {
        public StepEventMetadata Metadata { get; }
        public int Status { get; }
        public int Steps { get; }

        internal ResOpFailedEvent(StepEventMetadata metadata, int status, int steps)
        {
            Metadata = metadata;
            Status = status;
            Steps = steps;
        }
    }

    internal class EngineEvent
    {
        public int Sequence { get; }

        public int Timestamp { get; }

        public CancelEvent? CancelEvent { get; }
        public StdoutEngineEvent? StdoutEvent { get; }
        public DiagnosticEvent? DiagnosticEvent { get; }
        public PreludeEvent? PreludeEvent { get; }
        public SummaryEvent? SummaryEvent { get; }
        public ResourcePreEvent? ResourcePreEvent { get; }
        public ResOutputsEvent? ResOutputsEvent { get; }
        public ResOpFailedEvent? ResOpFailedEvent { get; }
        public PolicyEvent? PolicyEvent { get; }

        internal EngineEvent(
            int sequence,
            int timestamp,
            CancelEvent? cancelEvent,
            StdoutEngineEvent? stdoutEvent,
            DiagnosticEvent? diagnosticEvent,
            PreludeEvent? preludeEvent,
            SummaryEvent? summaryEvent,
            ResourcePreEvent? resourcePreEvent,
            ResOutputsEvent? resOutputsEvent,
            ResOpFailedEvent? resOpFailedEvent,
            PolicyEvent? policyEvent)
        {
            Sequence = sequence;
            Timestamp = timestamp;
            CancelEvent = cancelEvent;
            StdoutEvent = stdoutEvent;
            DiagnosticEvent = diagnosticEvent;
            PreludeEvent = preludeEvent;
            SummaryEvent = summaryEvent;
            ResourcePreEvent = resourcePreEvent;
            ResOutputsEvent = resOutputsEvent;
            ResOpFailedEvent = resOpFailedEvent;
            PolicyEvent = policyEvent;
        }
    }
}
