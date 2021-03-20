// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class ResOutputsEventModel : IJsonModel<ResOutputsEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public bool? Planning { get; set; } = null!;

        public ResOutputsEvent Convert() =>
            new ResOutputsEvent(
                this.Metadata.Convert(),
                this.Planning);
    }
}
