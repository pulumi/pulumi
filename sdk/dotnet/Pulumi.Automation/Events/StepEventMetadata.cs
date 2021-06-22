// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="StepEventMetadata"/> describes a "step" within the Pulumi engine, which is any concrete action
    /// to migrate a set of cloud resources from one state to another.
    /// </summary>
    public class StepEventMetadata
    {
        /// <summary>
        /// Op is the operation being performed.
        /// </summary>
        public OperationType Op { get; }

        public string Urn { get; }

        public string Type { get; }

        /// <summary>
        /// Old is the state of the resource before performing the step.
        /// </summary>
        public StepEventStateMetadata? Old { get; }

        /// <summary>
        /// New is the state of the resource after performing the step.
        /// </summary>
        public StepEventStateMetadata? New { get; }

        /// <summary>
        /// Keys causing a replacement (only applicable for "create" and "replace" Ops).
        /// </summary>
        public ImmutableArray<string>? Keys { get; }

        /// <summary>
        /// Keys that changed with this step.
        /// </summary>
        public ImmutableArray<string>? Diffs { get; }

        /// <summary>
        /// The diff for this step as a list of property paths and difference types.
        /// </summary>
        public IImmutableDictionary<string, PropertyDiff>? DetailedDiff { get; }

        /// <summary>
        /// Logical is set if the step is a logical operation in the program.
        /// </summary>
        public bool? Logical { get; }

        /// <summary>
        /// Provider actually performing the step.
        /// </summary>
        public string Provider { get; }

        internal StepEventMetadata(
            OperationType op,
            string urn,
            string type,
            StepEventStateMetadata? old,
            StepEventStateMetadata? @new,
            IEnumerable<string>? keys,
            IEnumerable<string>? diffs,
            IDictionary<string, PropertyDiff>? detailedDiff,
            bool? logical,
            string provider)
        {
            Op = op;
            Urn = urn;
            Type = type;
            Old = old;
            New = @new;
            Keys = keys?.ToImmutableArray();
            Diffs = diffs?.ToImmutableArray();
            DetailedDiff = detailedDiff?.ToImmutableDictionary();
            Logical = logical;
            Provider = provider;
        }
    }
}
