// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// ResourceTransformation is the callback signature for <see
    /// cref="ResourceOptions.ResourceTransformations"/>. A transformation is passed the same set of
    /// inputs provided to the <see cref="Resource"/> constructor, and can optionally return back
    /// alternate values for the <c>properties</c> and/or <c>options</c> prior to the resource
    /// actually being created. The effect will be as though those <c>properties</c> and/or
    /// <c>options</c> were passed in place of the original call to the <see cref="Resource"/>
    /// constructor.  If the transformation returns <see langword="null"/>, this indicates that the
    /// resource will not be transformed.
    /// </summary>
    /// <returns>The new values to use for the <c>args</c> and <c>options</c> of the <see
    /// cref="Resource"/> in place of the originally provided values.</returns>
    public delegate ResourceTransformationResult? ResourceTransformation(ResourceTransformationArgs args);

    public readonly struct ResourceTransformationArgs
    {
        /// <summary>
        /// The name of the Resource that is being transformed.
        /// </summary>
        public string Name { get; }
        /// <summary>
        /// The Resource instance that is being transformed.
        /// </summary>
        public Resource Resource { get; }
        /// <summary>
        /// The original properties passed to the Resource constructor.
        /// </summary>
        public ResourceArgs Args { get; }
        /// <summary>
        /// The original resource options passed to the Resource constructor.
        /// </summary>
        public ResourceOptions Options { get; }

        public ResourceTransformationArgs(
            Resource resource, string name, ResourceArgs args, ResourceOptions options)
        {
            Name = name;
            Resource = resource;
            Args = args;
            Options = options;
        }
    }

    public readonly struct ResourceTransformationResult
    {
        public string Name { get; }
        public ResourceArgs Args { get; }
        public ResourceOptions Options { get; }

        public ResourceTransformationResult(string name, ResourceArgs args, ResourceOptions options)
        {
            Name = name;
            Args = args;
            Options = options;
        }
    }
}
