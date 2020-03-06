// Copyright 2016-2020, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi.Utilities
{
    /// <summary>
    /// Allows extracting some internal insights about an instance of
    /// <see cref="Output{T}"/>. 
    /// </summary>
    public static class OutputUtilities
    {
        /// <summary>
        /// Retrieve the Is Known status of the given output.
        /// Note: generally, this should never be used in combination with await for
        /// a program control flow to avoid deadlock situations.
        /// </summary>
        /// <param name="output">The <see cref="Output{T}"/> to evaluate.</param>
        public static async Task<bool> GetIsKnownAsync<T>(Output<T> output)
        {
            var data = await output.DataTask.ConfigureAwait(false);
            return data.IsKnown;
        }

        /// <summary>
        /// Retrieve a set of resources that the given output depends on.
        /// </summary>
        /// <param name="output">The <see cref="Output{T}"/> to get dependencies of.</param>
        public static Task<ImmutableHashSet<Resource>> GetDependenciesAsync<T>(Output<T> output)
            => ((IOutput)output).GetResourcesAsync();
    }
}
