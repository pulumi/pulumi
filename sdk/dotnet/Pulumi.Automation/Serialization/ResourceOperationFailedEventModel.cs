// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class ResourceOperationFailedEventModel : IJsonModel<ResourceOperationFailedEvent>
    {
        public StepEventMetadataModel Metadata { get; set; } = null!;
        public int Status { get; set; }
        public int Steps { get; set; }

        public ResourceOperationFailedEvent Convert() =>
            new ResourceOperationFailedEvent(
                this.Metadata.Convert(),
                this.Status,
                this.Steps);
    }
}
