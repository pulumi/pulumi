// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Serialization.Json;
using Pulumi.Automation.Events;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class SummaryEventModel : IJsonModel<SummaryEvent>
    {
        public bool MaybeCorrupt { get; set; }

        public int DurationSeconds { get; set; }

        public Dictionary<OperationType, int> ResourceChanges { get; set; } = null!;

        public Dictionary<string, string> PolicyPacks { get; set; } = null!;

        public SummaryEvent Convert() =>
            new SummaryEvent(
                this.MaybeCorrupt,
                this.DurationSeconds,
                this.ResourceChanges ?? new Dictionary<OperationType, int>(),
                this.PolicyPacks ?? new Dictionary<string, string>());
    }
}
