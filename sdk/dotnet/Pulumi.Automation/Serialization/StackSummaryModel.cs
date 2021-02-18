// Copyright 2016-2021, Pulumi Corporation

using System;
using Pulumi.Automation.Serialization.Json;

namespace Pulumi.Automation.Serialization
{
    // necessary for constructor deserialization
    internal class StackSummaryModel : IJsonModel<StackSummary>
    {
        public string Name { get; set; } = null!;

        public bool Current { get; set; }

        public DateTimeOffset? LastUpdate { get; set; }

        public bool UpdateInProgress { get; set; }

        public int? ResourceCount { get; set; }

        public string? Url { get; set; }

        public StackSummary Convert()
            => new StackSummary(
                this.Name,
                this.Current,
                this.LastUpdate,
                this.UpdateInProgress,
                this.ResourceCount,
                this.Url);
    }
}
