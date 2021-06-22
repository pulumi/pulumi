// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="StandardOutputEngineEvent"/> is emitted whenever a generic message is written, for example warnings
    /// from the pulumi CLI itself. Less common than <see cref="DiagnosticEvent"/>.
    /// </summary>
    public class StandardOutputEngineEvent
    {
        public string Message { get; }

        public string Color { get; }

        internal StandardOutputEngineEvent(string message, string color)
        {
            Message = message;
            Color = color;
        }
    }
}
