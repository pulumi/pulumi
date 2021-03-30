// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="DiagnosticEvent"/> is emitted whenever a diagnostic message is provided, for example errors from
    /// a cloud resource provider while trying to create or update a resource.
    /// </summary>
    public class DiagnosticEvent
    {
        public string? Urn { get; }

        public string? Prefix { get; }

        public string Message { get; }

        public string Color { get; }

        /// <summary>
        /// Severity is one of "info", "info#err", "warning", or "error".
        /// </summary>
        public string Severity { get; }

        public string? StreamId { get; }

        public bool? Ephemeral { get; }

        internal DiagnosticEvent(
            string? urn,
            string? prefix,
            string message,
            string color,
            string severity,
            string? streamId,
            bool? ephemeral)
        {
            Urn = urn;
            Prefix = prefix;
            Message = message;
            Color = color;

            Severity = severity;
            StreamId = streamId;
            Ephemeral = ephemeral;
        }
    }
}
