// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class DiagnosticEventModel : IJsonModel<DiagnosticEvent>
    {
        public string? Urn { get; set; }

        public string? Prefix { get; set; }

        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public string Severity { get; set; } = null!;

        public string? StreamId { get; set; }

        public bool? Ephemeral { get; set; }

        public DiagnosticEvent Convert() =>
            new DiagnosticEvent(
                this.Urn,
                this.Prefix,
                this.Message,
                this.Color,
                this.Severity,
                this.StreamId,
                this.Ephemeral);
    }
}
