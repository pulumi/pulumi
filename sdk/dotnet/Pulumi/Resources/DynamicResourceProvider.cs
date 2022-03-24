// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    // TODO When we move to net6 and C# 9 these should probably be records

    /// <summary>
    /// CheckFailure represents a single failure in the results of a call to `DynamicResourceProvider.Check`
    /// </summary>
    public sealed class CheckFailure
    {
        /// <summary>
        /// The property that failed validation.
        /// </summary>
        public string Property { get; }

        /// <summary>
        /// The reason that the property failed validation.
        /// </summary>
        public string Reason { get; }

        public CheckFailure(string property, string reason)
        {
            Property = property;
            Reason = reason;
        }
    }

    /// <summary>
    /// CheckResult represents the results of a call to `DynamicResourceProvider.Check`.
    /// </summary>
    public sealed class CheckResult
    {
        /// <summary>
        /// The inputs to use, if any.
        /// </summary>
        public IDictionary<string, object?> Inputs { get; }

        /// <summary>
        /// Any validation failures that occurred.
        /// </summary>
        public IList<CheckFailure>? Failures { get; }

        public CheckResult(IDictionary<string, object?> inputs, IList<CheckFailure>? failures)
        {
            Inputs = inputs;
            Failures = failures;
        }
    }

    /// <summary>
    /// DiffResult represents the results of a call to `DynamicResourceProvider.Diff`.
    /// </summary>
    public sealed class DiffResult
    {
        /// <summary>
        /// If true, this diff detected changes and suggests an update.
        /// </summary>
        public Nullable<bool> Changes { get; }

        /// <summary>
        /// If this update requires a replacement, the set of properties triggering it.
        /// </summary>
        public IReadOnlyList<string>? Replaces { get; }

        /// <summary>
        /// An optional list of properties that will not ever change.
        /// </summary>
        public IReadOnlyList<string>? Stables { get; }

        /// <summary>
        /// If true, and a replacement occurs, the resource will first be deleted before being recreated.
        /// This is to void potential side-by-side issues with the default create before delete behavior.
        /// </summary>
        public bool DeleteBeforeReplace { get; }

        public DiffResult(Nullable<bool> changes = null, IEnumerable<string>? replaces = null, IEnumerable<string>? stables = null, bool deleteBeforeReplace = false)
        {
            this.Changes = changes;
            this.Replaces = replaces == null ? null : replaces.ToImmutableArray() as IReadOnlyList<string>;
            this.Stables = stables == null ? null : stables.ToImmutableArray() as IReadOnlyList<string>;
            this.DeleteBeforeReplace = deleteBeforeReplace;
        }
    }

    /// <summary>
    /// ResourceProvider is a Dynamic Resource Provider which allows defining new kinds of resources
    /// whose CRUD operations are implemented inside your Python program.
    /// </summary>
    public abstract class DynamicResourceProvider
    {
        /// <summary>
        /// Check validates that the given property bag is valid for a resource of the given type.
        /// </summary>
        public virtual Task<CheckResult> Check(ImmutableDictionary<string, object?> olds, ImmutableDictionary<string, object?> news)
        {
            return Task.FromResult(new CheckResult(news, null));
        }

        /// <summary>
        /// Diff checks what impacts a hypothetical update will have on the resource's properties.
        /// </summary>
        public virtual Task<DiffResult> Diff(string id, ImmutableDictionary<string, object?> olds, ImmutableDictionary<string, object?> news)
        {
            return Task.FromResult(new DiffResult());
        }

        /// <summary>
        /// Create allocates a new instance of the provided resource and returns its unique ID
        /// afterwards. If this call fails, the resource must not have been created (i.e., it is
        /// "transactional").
        /// </summary>
        public virtual Task<(string, IDictionary<string, object?>)> Create(ImmutableDictionary<string, object?> properties)
        {
            throw new NotImplementedException("Subclass of DynamicResourceProvider must implement 'create'");
        }

        /// <summary>
        /// Reads the current live state associated with a resource.  Enough state must be included in
        /// the inputs to uniquely identify the resource; this is typically just the resource ID, but it
        /// may also include some properties.
        /// </summary>
        public virtual Task<(string, IDictionary<string, object?>)> Read(string id, ImmutableDictionary<string, object?> properties)
        {
            return Task.FromResult<(string, IDictionary<string, object?>)>((id, properties));
        }

        /// <summary>
        /// Update updates an existing resource with new values.
        /// </summary>
        public virtual Task<IDictionary<string, object?>?> Update(string id, ImmutableDictionary<string, object?> olds, ImmutableDictionary<string, object?> news)
        {
            return Task.FromResult<IDictionary<string, object?>?>(null);
        }

        /// <summary>
        /// Delete tears down an existing resource with the given ID.  If it fails, the resource is
        /// assumed to still exist.
        /// </summary>
        public virtual Task Delete(string id, ImmutableDictionary<string, object?> properties)
        {
            return Task.CompletedTask;
        }
    }
}
