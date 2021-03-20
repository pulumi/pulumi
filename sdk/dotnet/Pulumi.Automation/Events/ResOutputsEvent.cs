// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
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
}
