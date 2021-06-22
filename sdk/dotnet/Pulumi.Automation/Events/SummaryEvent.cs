// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="SummaryEvent"/> is emitted at the end of an update, with a summary of the changes made.
    /// </summary>
    public class SummaryEvent
    {
        /// <summary>
        /// MaybeCorrupt is set if one or more of the resources is in an invalid state.
        /// </summary>
        public bool MaybeCorrupt { get; }

        /// <summary>
        /// Duration is the number of seconds the update was executing.
        /// </summary>
        public int DurationSeconds { get; }

        /// <summary>
        /// ResourceChanges contains the count for resource change by type.
        /// </summary>
        public IImmutableDictionary<OperationType, int> ResourceChanges { get; }

        /// <summary>
        /// PolicyPacks run during update. Maps PolicyPackName -> version.
        /// </summary>
        public IImmutableDictionary<string, string> PolicyPacks { get; }

        internal SummaryEvent(
            bool maybeCorrupt,
            int durationSeconds,
            IDictionary<OperationType, int> resourceChanges,
            IDictionary<string, string> policyPacks)
        {
            MaybeCorrupt = maybeCorrupt;
            DurationSeconds = durationSeconds;
            ResourceChanges = resourceChanges.ToImmutableDictionary();
            PolicyPacks = policyPacks.ToImmutableDictionary();
        }
    }
}
