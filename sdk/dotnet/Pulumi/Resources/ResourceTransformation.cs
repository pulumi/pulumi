// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;

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
    /// <returns>The new values to use for the <c>args</c> and <c>opts</c> of the <see
    /// cref="Resource"/> in place of the originally provided values.</returns>
    public delegate ResourceTransformationResult? ResourceTransformation(ResourceTransformationArgs args);

    public struct ResourceTransformationArgs
    {
        /// <summary>
        /// The Resource instance that is being transformed.
        /// </summary>
        public readonly Resource Resource;
        /// <summary>
        /// The original properties passed to the Resource constructor.
        /// </summary>
        public readonly ResourceArgs Args;
        /// <summary>
        /// The original resource options passed to the Resource constructor.
        /// </summary>
        public readonly ResourceOptions Opts;

        public ResourceTransformationArgs(
            Resource resource, ResourceArgs args, ResourceOptions opts)
        {
            Resource = resource;
            Args = args;
            Opts = opts;
        }
    }

    public struct ResourceTransformationResult
    {
        public readonly ResourceArgs Args;
        public readonly ResourceOptions Opts;

        public ResourceTransformationResult(ResourceArgs args, ResourceOptions opts)
        {
            Args = args;
            Opts = opts;
        }
    }
}