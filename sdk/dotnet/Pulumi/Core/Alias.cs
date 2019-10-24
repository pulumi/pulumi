// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi
{
    /// <summary>
    /// Alias is a partial description of prior named used for a resource. It can be processed in
    /// the context of a resource creation to determine what the full aliased URN would be.
    ///
    /// Note there is a semantic difference between properties being absent from this type and properties
    /// having the <see langword="null"/> value.Specifically, there is a difference between:
    ///
    /// <c>
    /// new Alias { Name = "foo", Parent = null } // and
    /// new Alias { Name = "foo" }
    /// </c>
    ///
    /// The presence of a property indicates if its value should be used. If absent, then the value
    /// is not used. So, in the above while <c>alias.Parent</c> is <see langword="null"/> for both,
    /// the first alias means "the original urn had no parent" while the second alias means "use the
    /// current parent".
    ///
    /// Note: to indicate that a resource was previously parented by the root stack, it is
    /// recommended that you use:
    ///
    /// <c>Aliases = { new Alias { Parent = Pulumi.Stack.Root } }</c>
    ///
    /// This form is self-descriptive and makes the intent clearer than using:
    ///
    /// <c>Aliases = { new Alias { Parent = null } }</c>
    /// </summary>
    public sealed class Alias
    {
        /// <summary>
        /// The previous name of the resource.  If not provided, the current name of the resource is
        /// used.
        /// </summary>
        public Optional<Input<string>> Name { get; set; }

        /// <summary>
        /// The previous type of the resource.  If not provided, the current type of the resource is used.
        /// </summary>
        public Optional<Input<string>> Type { get; set; }

        /// <summary>
        /// The previous stack of the resource.  If not provided, defaults to the value of <see
        /// cref="IDeployment.StackName"/>.
        /// </summary>
        public Optional<Input<string>> Stack { get; set; }

        /// <summary>
        /// The previous project of the resource. f not provided, defaults to the value of <see
        /// cref="IDeployment.ProjectName"/>.
        /// </summary>
        public Optional<Input<string>> Project { get; set; }

        /// <summary>
        /// The previous parent of the resource. If not provided (i.e. <c>new Alias { Name =
        /// "foo"}</c>), the current parent of the resource is used (<c>options.Parent</c> if provided,
        /// else the implicit stack resource parent).
        /// 
        /// To specify no original parent, use <c>new Alias { Parent = Pulumi.Stack.Root }</c>.
        /// 
        /// Only specify one of <see cref="Parent"/> or <see cref="ParentUrn"/>.
        /// </summary>
        public Optional<Resource?> Parent { get; set; }

        /// <summary>
        /// The previous parent of the resource. If not provided (i.e. <c>new Alias { Name =
        /// "foo"}</c>), the current parent of the resource is used (<c>options.Parent</c> if provided,
        /// else the implicit stack resource parent).
        /// 
        /// To specify no original parent, use <c>new Alias { Parent = Pulumi.Stack.Root }</c>.
        /// 
        /// Only specify one of <see cref="Parent"/> or <see cref="ParentUrn"/>.
        /// </summary>
        public Optional<Input<string>> ParentUrn { get; set; }
    }
}
