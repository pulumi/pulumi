// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
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
}
