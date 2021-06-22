// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="PolicyEvent"/> is emitted whenever there is Policy violation.
    /// </summary>
    public class PolicyEvent
    {
        public string? ResourceUrn { get; }

        public string Message { get; }

        public string Color { get; }

        public string PolicyName { get; }

        public string PolicyPackName { get; }

        public string PolicyPackVersion { get; }

        public string PolicyPackVersionTag { get; }

        /// <summary>
        /// EnforcementLevel is one of "warning" or "mandatory"
        /// </summary>
        public string EnforcementLevel { get; }

        internal PolicyEvent(
            string? resourceUrn,
            string message,
            string color,
            string policyName,
            string policyPackName,
            string policyPackVersion,
            string policyPackVersionTag,
            string enforcementLevel)
        {
            ResourceUrn = resourceUrn;
            Message = message;
            Color = color;
            PolicyName = policyName;
            PolicyPackName = policyPackName;
            PolicyPackVersion = policyPackVersion;
            PolicyPackVersionTag = policyPackVersionTag;
            EnforcementLevel = enforcementLevel;
        }
    }
}
