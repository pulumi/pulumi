// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi.Utilities
{
    /// <summary>
    /// Allows extracting some internal insights about an instance of
    /// <see cref="Output{T}"/>.
    /// 
    /// Danger: these utilities are intended for use in test and
    /// debugging scenarios. In normal Pulumi programs, please
    /// consider using `.Apply` instead to chain `Output{T}`
    /// transformations without unpacking the underlying T. Doing
    /// so preserves metadata such as resource dependencies that
    /// is used by Pulumi engine to operate correctly. Using
    /// `await output.GetValueAsync()` directly opens up a possibility
    /// to introduce issues with lost metadata.
    /// </summary>
    public static class OutputUtilities
    {
        /// <summary>
        /// Create an unknown with the given value.
        /// Note: generally, this should never be used since an unknown never resolves during preview.
        /// Bearing that in mind, this can be used in combination with await for
        /// a program control flow to avoid deadlock situations.
        /// </summary>
        /// <param name="value">The value.</param>
        public static Output<T> CreateUnknown<T>(T value)
            => Output<T>.CreateUnknown(value);

        /// <summary>
        /// Create an unknown with the given value factory.
        /// Note: generally, this should never be used since an unknown never resolves during preview.
        /// Bearing that in mind, this can be used in combination with await for
        /// a program control flow to avoid deadlock situations.
        /// In particular, the value factory will never be called during preview.
        /// </summary>
        /// <param name="valueFactory">The value factory.</param>
        public static Output<T> CreateUnknown<T>(Func<Task<T>> valueFactory)
            => Output<T>.CreateUnknown(valueFactory);

        /// <summary>
        /// Retrieve the known status of the given output.
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
        /// Retrieve the value of the given output.
        /// Note: generally, this should never be used in combination with await for
        /// a program control flow to avoid deadlock situations.
        /// </summary>
        /// <param name="output">The <see cref="Output{T}"/> to evaluate.</param>
        public static Task<T> GetValueAsync<T>(Output<T> output)
            => output.GetValueAsync();

        /// <summary>
        /// Retrieve a set of resources that the given output depends on.
        /// </summary>
        /// <param name="output">The <see cref="Output{T}"/> to get dependencies of.</param>
        public static Task<ImmutableHashSet<Resource>> GetDependenciesAsync<T>(Output<T> output)
            => ((IOutput)output).GetResourcesAsync();
    }
}
