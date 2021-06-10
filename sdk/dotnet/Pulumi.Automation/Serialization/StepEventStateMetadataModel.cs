// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
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
}
