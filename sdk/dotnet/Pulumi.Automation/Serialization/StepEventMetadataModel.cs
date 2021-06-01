// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using System.Linq;
using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class StepEventMetadataModel : IJsonModel<StepEventMetadata>
    {
        public OperationType Op { get; set; }

        public string Urn { get; set; } = null!;

        public string Type { get; set; } = null!;

        public StepEventStateMetadataModel? Old { get; set; } = null!;

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
}
