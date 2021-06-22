// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="CancelEvent"/> is emitted when the user initiates a cancellation of the update in progress, or
    /// the update successfully completes.
    /// </summary>
    public class CancelEvent
    {
        internal CancelEvent() {}
    }
}
