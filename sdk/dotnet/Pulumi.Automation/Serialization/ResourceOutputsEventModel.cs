// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class ResourceOutputsEventModel : IJsonModel<ResourceOutputsEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public bool? Planning { get; set; } = null!;

        public ResourceOutputsEvent Convert() =>
            new ResourceOutputsEvent(
                this.Metadata.Convert(),
                this.Planning);
    }
}
