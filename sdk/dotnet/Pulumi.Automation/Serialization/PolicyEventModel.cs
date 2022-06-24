// Copyright 2016-2021, Pulumi Corporation

using Pulumi.Automation.Events;
using Pulumi.Automation.Serialization.Json;

// NOTE: The classes in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go
namespace Pulumi.Automation.Serialization
{
    internal class PolicyEventModel : IJsonModel<PolicyEvent>
    {
        public string? ResourceUrn { get; set; } = null!;

        public string Message { get; set; } = null!;

        public string Color { get; set; } = null!;

        public string PolicyName { get; set; } = null!;

        public string PolicyPackName { get; set; } = null!;

        public string PolicyPackVersion { get; set; } = null!;

        public string PolicyPackVersionTag { get; set; } = null!;

        public string EnforcementLevel { get; set; } = null!;

        public PolicyEvent Convert() =>
            new PolicyEvent(
                this.ResourceUrn,
                this.Message,
                this.Color,
                this.PolicyName,
                this.PolicyPackName,
                this.PolicyPackVersion,
                this.PolicyPackVersionTag,
                this.EnforcementLevel);
    }
}
