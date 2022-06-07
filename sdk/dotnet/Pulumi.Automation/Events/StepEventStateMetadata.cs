// Copyright 2016-2021, Pulumi Corporation
// NOTE: The class in this file are intended to align with the serialized
// JSON types defined and versioned in sdk/go/common/apitype/events.go

using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi.Automation.Events
{
    /// <summary>
    /// <see cref="StepEventStateMetadata"/> is the more detailed state information for a resource as it relates to
    /// a step(s) being performed.
    /// </summary>
    public class StepEventStateMetadata
    {
        public string Urn { get; }

        public string Type { get; }

        /// <summary>
        /// Custom indicates if the resource is managed by a plugin.
        /// </summary>
        public bool? Custom { get; }

        /// <summary>
        /// Delete is true when the resource is pending deletion due to a replacement.
        /// </summary>
        public bool? Delete { get; }

        /// <summary>
        /// ID is the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
        /// </summary>
        public string Id { get; }

        /// <summary>
        /// Parent is an optional parent URN that this resource belongs to.
        /// </summary>
        public string Parent { get; }

        /// <summary>
        /// Protect is true to "protect" this resource (protected resources cannot be deleted).
        /// </summary>
        public bool? Protect { get; }

        /// <summary>
        /// Inputs contains the resource's input properties (as specified by the program). Secrets have
        /// filtered out, and large assets have been replaced by hashes as applicable.
        /// </summary>
        public IImmutableDictionary<string, object> Inputs { get; }

        /// <summary>
        /// Outputs contains the resource's complete output state (as returned by the resource provider).
        /// </summary>
        public IImmutableDictionary<string, object> Outputs { get; }

        /// <summary>
        /// Provider is the resource's provider reference
        /// </summary>
        public string Provider { get; }

        /// <summary>
        /// InitErrors is the set of errors encountered in the process of initializing resource.
        /// </summary>
        public ImmutableArray<string>? InitErrors { get; }

        internal StepEventStateMetadata(
            string urn,
            string type,
            bool? custom,
            bool? delete,
            string id,
            string parent,
            bool? protect,
            IDictionary<string, object> inputs,
            IDictionary<string, object> outputs,
            string provider,
            IEnumerable<string>? initErrors)
        {
            Urn = urn;
            Type = type;
            Custom = custom;
            Delete = delete;
            Id = id;
            Parent = parent;
            Protect = protect;
            Inputs = inputs.ToImmutableDictionary();
            Outputs = outputs.ToImmutableDictionary();
            Provider = provider;
            InitErrors = initErrors?.ToImmutableArray();
        }
    }
}
