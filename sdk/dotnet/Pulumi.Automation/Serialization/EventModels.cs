// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;
using System.Linq;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class CancelEventModel : IJsonModel<CancelEvent>
    {
        public CancelEvent Convert() =>
            new CancelEvent();
    }

    internal class StdoutEngineEventModel : IJsonModel<StdoutEngineEvent>
    {
        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public StdoutEngineEvent Convert() =>
            new StdoutEngineEvent(this.Message, this.Color);
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
                this.Urn,
                this.Prefix,
                this.Message,
                this.Color,
                this.Severity,
                this.StreamId,
                this.Ephemeral);
    }

    internal class PolicyEventModel : IJsonModel<PolicyEvent>
    {
        public string? ResourceUrn { get; set; } = null!;

        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public string PolicyName { get; set; } = null!;

        public string PolicyPackName { get; set; } = null!;

        public string PolicyPackVersion { get; set; } = null!;

        public string PolicyPackVersionTag { get; set; } = null!;

        public string EnforcementLevel { get; set; } = null!;

        public PolicyEvent Convert() =>
            new PolicyEvent(
                this.ResourceUrn,
                this.Message,
                this.Color,
                this.PolicyName,
                this.PolicyPackName,
                this.PolicyPackVersion,
                this.PolicyPackVersionTag,
                this.EnforcementLevel);
    }

    internal class PreludeEventModel : IJsonModel<PreludeEvent>
    {
        public Dictionary<string, string> Config { get; set; } = null!;

        public PreludeEvent Convert() => new PreludeEvent(this.Config);
    }

    internal class SummaryEventModel : IJsonModel<SummaryEvent>
    {
        public bool MaybeCorrupt { get; set; }

        public int DurationSeconds { get; set; }

        public Dictionary<OperationType, int> ResourceChanges { get; set; } = null!;

        public Dictionary<string, string> PolicyPacks { get; set; } = null!;

        public SummaryEvent Convert() =>
            new SummaryEvent(
                this.MaybeCorrupt,
                this.DurationSeconds,
                this.ResourceChanges ?? new Dictionary<OperationType, int>(),
                this.PolicyPacks ?? new Dictionary<string, string>());
    }

    internal class PropertyDiffModel : IJsonModel<PropertyDiff>
    {
        public DiffKind Kind { get; set; }

        public bool InputDiff { get; set; }

        public PropertyDiff Convert() =>
            new PropertyDiff(this.Kind, this.InputDiff);
    }

    internal class StepEventMetadataModel : IJsonModel<StepEventMetadata>
    {
        public OperationType Op { get; set; }

        public string Urn { get; set; } = null!;

        public string Type { get; set; } = null!;

        public StepEventStateMetadataModel? Old { get; set; } = null!;

        // TODO: can this ever be null?
        public StepEventStateMetadataModel? New { get; set; } = null!;

        public List<string>? Keys { get; set; } = null!;

        public List<string>? Diffs { get; set; } = null!;

        public Dictionary<string, PropertyDiffModel>? DetailedDiff { get; set; } = null!;

        public bool? Logical { get; set; } = null!;

        public string Provider { get; set; } = null!;

        public StepEventMetadata Convert() =>
            new StepEventMetadata(
                this.Op,
                this.Urn,
                this.Type,
                this.Old?.Convert(),
                this.New?.Convert(),
                this.Keys,
                this.Diffs,
                this.DetailedDiff?.ToDictionary(kvp => kvp.Key, kvp => kvp.Value.Convert()),
                this.Logical,
                this.Provider);
    }

    internal class StepEventStateMetadataModel : IJsonModel<StepEventStateMetadata>
    {
        public string Urn { get; set; } = null!;

        public string Type { get; set; } = null!;

        public bool? Custom { get; set; } = null!;

        public bool? Delete { get; set; } = null!;

        public string Id { get; set; } = null!;

        public string Parent { get; set; } = null!;

        public bool? Protect { get; set; } = null!;

        public Dictionary<string, object> Inputs { get; set; } = null!;

        public Dictionary<string, object> Outputs { get; set; } = null!;

        public string Provider { get; set; } = null!;

        public List<string>? InitErrors { get; set; } = null!;

        public StepEventStateMetadata Convert() =>
            new StepEventStateMetadata(
                this.Urn,
                this.Type,
                this.Custom,
                this.Delete,
                this.Id,
                this.Parent,
                this.Protect,
                this.Inputs,
                this.Outputs,
                this.Provider,
                this.InitErrors);
    }

    internal class ResourcePreEventModel : IJsonModel<ResourcePreEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public bool? Planning { get; set; } = null!;

        public ResourcePreEvent Convert() =>
            new ResourcePreEvent(
                this.Metadata.Convert(),
                this.Planning);
    }

    internal class ResOutputsEventModel : IJsonModel<ResOutputsEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public bool? Planning { get; set; } = null!;

        public ResOutputsEvent Convert() =>
            new ResOutputsEvent(
                this.Metadata.Convert(),
                this.Planning);
    }

    internal class ResOpFailedEventModel : IJsonModel<ResOpFailedEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public int Status { get; set; }
        public int Steps { get; set; }

        public ResOpFailedEvent Convert() =>
            new ResOpFailedEvent(
                this.Metadata.Convert(),
                this.Status,
                this.Steps);
    }

    internal class EngineEventModel : IJsonModel<EngineEvent>
    {
        public int Sequence { get; set; }

        public int Timestamp { get; set; }

        public CancelEventModel? CancelEvent { get; set; }
        public StdoutEngineEventModel? StdoutEvent { get; set; }
        public DiagnosticEventModel? DiagnosticEvent { get; set; }
        public PreludeEventModel? PreludeEvent { get; set; }
        public SummaryEventModel? SummaryEvent { get; set; }
        public ResourcePreEventModel? ResourcePreEvent { get; set; }
        public ResOutputsEventModel? ResOutputsEvent { get; set; }
        public ResOpFailedEventModel? ResOpFailedEvent { get; set; }
        public PolicyEventModel? PolicyEvent { get; set; }

        public EngineEvent Convert() =>
            new EngineEvent(
                this.Sequence,
                this.Timestamp,
                this.CancelEvent?.Convert(),
                this.StdoutEvent?.Convert(),
                this.DiagnosticEvent?.Convert(),
                this.PreludeEvent?.Convert(),
                this.SummaryEvent?.Convert(),
                this.ResourcePreEvent?.Convert(),
                this.ResOutputsEvent?.Convert(),
                this.ResOpFailedEvent?.Convert(),
                this.PolicyEvent?.Convert());
    }
}
