// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="ResourceOutputsEvent"/> is emitted when a resource is finished being provisioned.
    /// </summary>
    public class ResourceOutputsEvent
    {
        public StepEventMetadata Metadata { get; }
        public bool? Planning { get; }

        internal ResourceOutputsEvent(StepEventMetadata metadata, bool? planning)
        {
            Metadata = metadata;
            Planning = planning;
        }
    }
}
