// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using Pulumi.Automation.Serialization.Json;

namespace Pulumi.Automation.Serialization
{
    // necessary for constructor deserialization
    internal class UpdateSummaryModel : IJsonModel<UpdateSummary>
    {
        // pre-update information
        public UpdateKind Kind { get; set; }

        public DateTimeOffset StartTime { get; set; }

        public string? Message { get; set; }

        public Dictionary<string, string>? Environment { get; set; }

        public Dictionary<string, ConfigValue>? Config { get; set; }

        // post-update information
        public string? Result { get; set; }

        public DateTimeOffset EndTime { get; set; }

        public int? Version { get; set; }

        public string? Deployment { get; set; }

        public Dictionary<OperationType, int>? ResourceChanges { get; set; }

        private UpdateState GetResult()
            => string.Equals(this.Result, "not-started", StringComparison.OrdinalIgnoreCase) ? UpdateState.NotStarted
            : string.Equals(this.Result, "in-progress", StringComparison.OrdinalIgnoreCase) ? UpdateState.InProgress
            : string.Equals(this.Result, "succeeded", StringComparison.OrdinalIgnoreCase) ? UpdateState.Succeeded
            : string.Equals(this.Result, "failed", StringComparison.OrdinalIgnoreCase) ? UpdateState.Failed
            : throw new InvalidOperationException($"Invalid update result: {this.Result}");

        public UpdateSummary Convert()
        {
            return new UpdateSummary(
                this.Kind,
                this.StartTime,
                this.Message ?? string.Empty,
                this.Environment ?? new Dictionary<string, string>(),
                this.Config ?? new Dictionary<string, ConfigValue>(),
                this.GetResult(),
                this.EndTime,
                this.Version,
                this.Deployment,
                this.ResourceChanges);
        }
    }
}
