// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;

namespace Pulumi
{
    /// <summary>
    /// ResourceTransformation is the callback signature for <see
    /// cref="ResourceOptions.ResourceTransformations"/>. A transformation is passed the same set of
    /// inputs provided to the <see cref="Resource"/> constructor, and can optionally return back
    /// alternate values for the <c>properties</c> and/or <c>options</c> prior to the resource
    /// actually being created. The effect will be as though those <c>properties</c> and/or
    /// <c>options</c> were passed in place of the original call to the <see cref="Resource"/>
    /// constructor.  If the transformation returns undefined, this indicates that the resource will
    /// not be transformed.
    /// </summary>
    /// <param name="resource">The Resource instance that is being transformed.</param>
    /// <param name="type">The type of the Resource.</param>
    /// <param name="name">The name of the Resource.</param>
    /// <param name="args">The original properties passed to the Resource constructor.</param>
    /// <param name="opts">The original resource options passed to the Resource constructor.</param>
    /// <returns>The new values to use for the `properties` and `options` of the <see
    /// cref="Resource"/> in place of the originally provided values.</returns>
    public delegate (ResourceArgs args, ResourceOptions opts)? ResourceTransformation(
        Resource resource, string type, string name,
        ResourceArgs args, ResourceOptions opts);
}