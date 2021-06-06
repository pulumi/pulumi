// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;
using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

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
                // ReSharper disable once ConstantNullCoalescingCondition
                this.ResourceChanges ?? new Dictionary<OperationType, int>(),
                // ReSharper disable once ConstantNullCoalescingCondition
                this.PolicyPacks ?? new Dictionary<string, string>());
    }
}
