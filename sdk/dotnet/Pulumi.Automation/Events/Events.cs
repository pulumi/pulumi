// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Events
{
    /// <summary>
    /// CancelEvent is emitted when the user initiates a cancellation of the update in progress, or
    /// the update successfully completes.
    /// </summary>
    public class CancelEvent
    {
        internal CancelEvent() {}
    }

    /// <summary>
    /// StdoutEngineEvent is emitted whenever a generic message is written, for example warnings
    /// from the pulumi CLI itself. Less common than DiagnosticEvent.
    /// </summary>
    public class StdoutEngineEvent
    {
        public string Message { get; }

        public string Color { get; }

        internal StdoutEngineEvent(string message, string color)
        {
            Message = message;
            Color = color;
        }
    }

    /// <summary>
    /// DiagnosticEvent is emitted whenever a diagnostic message is provided, for example errors from
    /// a cloud resource provider while trying to create or update a resource.
    /// </summary>
    public class DiagnosticEvent
    {
        public string? Urn { get; }

        public string? Prefix { get; }

        public string Message { get; }

        public string Color { get; }

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

    /// <summary>
    /// PolicyEvent is emitted whenever there is Policy violation.
    /// </summary>
    public class PolicyEvent
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

    /// <summary>
    /// PreludeEvent is emitted at the start of an update.
    /// </summary>
    public class PreludeEvent
    {
        /// <summary>
        /// Config contains the keys and values for the update.
        /// Encrypted configuration values may be blinded.
        /// </summary>
        public IImmutableDictionary<string, string> Config { get; }

        internal PreludeEvent(IDictionary<string, string> config)
        {
            Config = config.ToImmutableDictionary();
        }
    }

    /// <summary>
    /// SummaryEvent is emitted at the end of an update, with a summary of the changes made.
    /// </summary>
    public class SummaryEvent
    {
        /// <summary>
        /// MaybeCorrupt is set if one or more of the resources is in an invalid state.
        /// </summary>
        public bool MaybeCorrupt { get; }

        /// <summary>
        /// Duration is the number of seconds the update was executing.
        /// </summary>
        public int DurationSeconds { get; }

        /// <summary>
        /// ResourceChanges contains the count for resource change by type.
        /// </summary>
        public IImmutableDictionary<OperationType, int> ResourceChanges { get; }

        /// <summary>
        /// PolicyPacks run during update. Maps PolicyPackName -> version.
        /// </summary>
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

    /// <summary>
    /// DiffKind describes the kind of a particular property diff.
    /// </summary>
    public enum DiffKind
    {
        /// <summary>
        /// Add indicates that the property was added.
        /// </summary>
        Add,
        /// <summary>
        /// AddReplace indicates that the property was added and requires that the resource be replaced.
        /// </summary>
        AddReplace,
        /// <summary>
        /// Delete indicates that the property was deleted.
        /// </summary>
        Delete,
        /// <summary>
        /// DeleteReplace indicates that the property was deleted and requires that the resource be replaced.
        /// </summary>
        DeleteReplace,
        /// <summary>
        /// Update indicates that the property was updated.
        /// </summary>
        Update,
        /// <summary>
        /// UpdateReplace indicates that the property was updated and requires that the resource be replaced.
        /// </summary>
        UpdateReplace,
    }

    /// <summary>
    /// PropertyDiff describes the difference between a single property's old and new values.
    /// </summary>
    public class PropertyDiff
    {
        /// <summary>
        /// Kind is the kind of difference.
        /// </summary>
        public DiffKind Kind { get; }

        /// <summary>
        /// InputDiff is true if this is a difference between old and new inputs rather than old state and new inputs.
        /// </summary>
        public bool InputDiff { get; }

        internal PropertyDiff(DiffKind kind, bool inputDiff)
        {
            Kind = kind;
            InputDiff = inputDiff;
        }
    }

    /// <summary>
    /// StepEventMetadata describes a "step" within the Pulumi engine, which is any concrete action
    /// to migrate a set of cloud resources from one state to another.
    /// </summary>
    public class StepEventMetadata
    {
        /// <summary>
        /// Op is the operation being performed.
        /// </summary>
        public OperationType Op { get; }

        public string Urn { get; }

        public string Type { get; }

        /// <summary>
        /// Old is the state of the resource before performing the step.
        /// </summary>
        public StepEventStateMetadata? Old { get; }

        /// <summary>
        /// New is the state of the resource after performing the step.
        /// </summary>
        public StepEventStateMetadata? New { get; }

        /// <summary>
        /// Keys causing a replacement (only applicable for "create" and "replace" Ops).
        /// </summary>
        public ImmutableArray<string>? Keys { get; }

        /// <summary>
        /// Keys that changed with this step.
        /// </summary>
        public ImmutableArray<string>? Diffs { get; }

        /// <summary>
        /// The diff for this step as a list of property paths and difference types.
        /// </summary>
        public IImmutableDictionary<string, PropertyDiff>? DetailedDiff { get; }

        /// <summary>
        /// Logical is set if the step is a logical operation in the program.
        /// </summary>
        public bool? Logical { get; }

        /// <summary>
        /// Provider actually performing the step.
        /// </summary>
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

    /// <summary>
    /// StepEventStateMetadata is the more detailed state information for a resource as it relates to
    /// a step(s) being performed.
    /// </summary>
    public class StepEventStateMetadata
    {
        public string Urn { get; }

        public string Type { get; }

        /// <summary>
        /// Custom indicates if the resource is managed by a plugin.
        /// </summary>
        public bool? Custom { get; }

        /// <summary>
        /// Delete is true when the resource is pending deletion due to a replacement.
        /// </summary>
        public bool? Delete { get; }

        /// <summary>
        /// ID is the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
        /// </summary>
        public string Id { get; }

        /// <summary>
        /// Parent is an optional parent URN that this resource belongs to.
        /// </summary>
        public string Parent { get; }

        /// <summary>
        /// Protect is true to "protect" this resource (protected resources cannot be deleted).
        /// </summary>
        public bool? Protect { get; }

        /// <summary>
        /// Inputs contains the resource's input properties (as specified by the program). Secrets have
        /// filtered out, and large assets have been replaced by hashes as applicable.
        /// </summary>
        public IImmutableDictionary<string, object> Inputs { get; }

        /// <summary>
        /// Outputs contains the resource's complete output state (as returned by the resource provider).
        /// </summary>
        public IImmutableDictionary<string, object> Outputs { get; }

        /// <summary>
        /// Provider is the resource's provider reference
        /// </summary>
        public string Provider { get; }

        /// <summary>
        /// InitErrors is the set of errors encountered in the process of initializing resource.
        /// </summary>
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

    /// <summary>
    /// ResourcePreEvent is emitted before a resource is modified.
    /// </summary>
    public class ResourcePreEvent
    {
        public StepEventMetadata Metadata { get; }
        public bool? Planning { get; }

        internal ResourcePreEvent(StepEventMetadata metadata, bool? planning)
        {
            Metadata = metadata;
            Planning = planning;
        }
    }

    /// <summary>
    /// ResOutputsEvent is emitted when a resource is finished being provisioned.
    /// </summary>
    public class ResOutputsEvent
    {
        public StepEventMetadata Metadata { get; }
        public bool? Planning { get; }

        internal ResOutputsEvent(StepEventMetadata metadata, bool? planning)
        {
            Metadata = metadata;
            Planning = planning;
        }
    }

    /// <summary>
    /// ResOpFailedEvent is emitted when a resource operation fails. Typically a DiagnosticEvent is
    /// emitted before this event, indiciating what the root cause of the error.
    /// </summary>
    public class ResOpFailedEvent
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

    /// <summary>
    /// EngineEvent describes a Pulumi engine event, such as a change to a resource or diagnostic
    /// message. EngineEvent is a discriminated union of all possible event types, and exactly one
    /// field will be non-null.
    /// </summary>
    public class EngineEvent
    {
        /// <summary>
        /// Sequence is a unique, and monotonically increasing number for each engine event sent to the
        /// Pulumi Service. Since events may be sent concurrently, and/or delayed via network routing,
        /// the sequence number is to ensure events can be placed into a total ordering.
        /// <para>
        /// - No two events can have the same sequence number.
        /// </para>
        /// <para>
        /// - Events with a lower sequence number must have been emitted before those with a higher sequence number.
        /// </para>
        /// </summary>
        public int Sequence { get; }

        /// <summary>
        /// Timestamp is a Unix timestamp (seconds) of when the event was emitted.
        /// </summary>
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
