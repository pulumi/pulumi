// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Alias is a partial description of prior named used for a resource. It can be processed in
    /// the context of a resource creation to determine what the full aliased URN would be.
    /// <para/>
    /// The presence of a property indicates if its value should be used. If absent (i.e.
    /// <see langword="null"/>), then the value is not used.
    /// <para/>
    /// Note: because of the above, there needs to be special handling to indicate that the previous
    /// <see cref="Parent"/> of a <see cref="Resource"/> was <see langword="null"/>.  Specifically,
    /// pass in:
    /// <para/>
    /// <c>Aliases = { new Alias { NoParent = true } }</c>
    /// </summary>
    public sealed class Alias
    {
        /// <summary>
        /// The previous name of the resource.  If <see langword="null"/>, the current name of the
        /// resource is used.
        /// </summary>
        public Input<string>? Name { get; set; }

        /// <summary>
        /// The previous type of the resource.  If <see langword="null"/>, the current type of the
        /// resource is used.
        /// </summary>
        public Input<string>? Type { get; set; }

        /// <summary>
        /// The previous stack of the resource.  If <see langword="null"/>, defaults to the value of
        /// <see cref="IDeployment.StackName"/>.
        /// </summary>
        public Input<string>? Stack { get; set; }

        /// <summary>
        /// The previous project of the resource. If <see langword="null"/>, defaults to the value
        /// of <see cref="IDeployment.ProjectName"/>.
        /// </summary>
        public Input<string>? Project { get; set; }

        /// <summary>
        /// The previous parent of the resource. If <see langword="null"/>, the current parent of
        /// the resource is used.
        /// <para/>
        /// To specify no original parent, use <c>new Alias { NoParent = true }</c>.
        /// <para/>
        /// Only specify one of <see cref="Parent"/> or <see cref="ParentUrn"/> or <see cref="NoParent"/>.
        /// </summary>
        public Resource? Parent { get; set; }

        /// <summary>
        /// The previous parent of the resource. If <see langword="null"/>, the current parent of
        /// the resource is used.
        /// <para/>
        /// To specify no original parent, use <c>new Alias { NoParent = true }</c>.
        /// <para/>
        /// Only specify one of <see cref="Parent"/> or <see cref="ParentUrn"/> or <see cref="NoParent"/>.
        /// </summary>
        public Input<string>? ParentUrn { get; set; }

        /// <summary>
        /// Used to indicate the resource previously had no parent.  If <see langword="false"/> this
        /// property is ignored.
        /// <para/>
        /// To specify no original parent, use <c>new Alias { NoParent = true }</c>.
        /// <para/>
        /// Only specify one of <see cref="Parent"/> or <see cref="ParentUrn"/> or <see cref="NoParent"/>.
        /// </summary>
        public bool NoParent { get; set; }
    }
}
