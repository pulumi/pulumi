// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="PropertyDiff"/> describes the difference between a single property's old and new values.
    /// </summary>
    public class PropertyDiff
    {
        /// <summary>
        /// Kind is the kind of difference.
        /// </summary>
        public DiffKind Kind { get; }

        /// <summary>
        /// InputDiff is true if this is a difference between old and new inputs rather than old state and new inputs.
        /// </summary>
        public bool InputDiff { get; }

        internal PropertyDiff(DiffKind kind, bool inputDiff)
        {
            Kind = kind;
            InputDiff = inputDiff;
        }
    }
}
