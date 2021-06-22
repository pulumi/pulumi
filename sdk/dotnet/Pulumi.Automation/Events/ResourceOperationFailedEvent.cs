// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="ResourceOperationFailedEvent"/> is emitted when a resource operation fails. Typically a <see cref="DiagnosticEvent"/> is
    /// emitted before this event, indiciating what the root cause of the error.
    /// </summary>
    public class ResourceOperationFailedEvent
    {
        public StepEventMetadata Metadata { get; }
        public int Status { get; }
        public int Steps { get; }

        internal ResourceOperationFailedEvent(StepEventMetadata metadata, int status, int steps)
        {
            Metadata = metadata;
            Status = status;
            Steps = steps;
        }
    }
}
