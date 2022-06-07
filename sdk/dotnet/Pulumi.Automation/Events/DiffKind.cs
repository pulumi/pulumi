// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="DiffKind"/> describes the kind of a particular property diff.
    /// </summary>
    public enum DiffKind
    {
        /// <summary>
        /// Add indicates that the property was added.
        /// </summary>
        Add,
        /// <summary>
        /// AddReplace indicates that the property was added and requires that the resource be replaced.
        /// </summary>
        AddReplace,
        /// <summary>
        /// Delete indicates that the property was deleted.
        /// </summary>
        Delete,
        /// <summary>
        /// DeleteReplace indicates that the property was deleted and requires that the resource be replaced.
        /// </summary>
        DeleteReplace,
        /// <summary>
        /// Update indicates that the property was updated.
        /// </summary>
        Update,
        /// <summary>
        /// UpdateReplace indicates that the property was updated and requires that the resource be replaced.
        /// </summary>
        UpdateReplace,
    }
}
