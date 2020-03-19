// Copyright 2016-2020, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi.Testing
{
    /// <summary>
    /// Hooks to mock the engine that provide test doubles for offline unit testing of stacks.
    /// </summary>
    public interface IMocks
    {
        /// <summary>
        /// Invoked when a new resource is created by the program.
        /// </summary>
        /// <param name="type">Resource type name.</param>
        /// <param name="name">Resource name.</param>
        /// <param name="inputs">Dictionary of resource input properties.</param>
        /// <param name="provider">Provider.</param>
        /// <param name="id">Resource identifier.</param>
        /// <returns>A tuple of a resource identifier and resource state. State can be either a POCO
        /// or a dictionary bag.</returns>
        Task<(string id, object state)> NewResourceAsync(string type, string name,
            ImmutableDictionary<string, object> inputs, string? provider, string? id);

        /// <summary>
        /// Invoked when the program needs to call a provider to load data (e.g., to retrieve an existing
        /// resource).
        /// </summary>
        /// <param name="token">Function token.</param>
        /// <param name="args">Dictionary of input arguments.</param>
        /// <param name="provider">Provider.</param>
        /// <returns>Invocation result, can be either a POCO or a dictionary bag.</returns>
        Task<object> CallAsync(string token, ImmutableDictionary<string, object> args, string? provider);
    }
}
