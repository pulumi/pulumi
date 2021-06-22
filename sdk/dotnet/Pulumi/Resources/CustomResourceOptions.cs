// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// <see cref="CustomResourceOptions"/> is a bag of optional settings that control a <see
    /// cref="CustomResource"/>'s behavior.
    /// </summary>
    public sealed class CustomResourceOptions : ResourceOptions
    {
        /// <summary>
        /// When set to <c>true</c>, indicates that this resource should be deleted before its
        /// replacement is created when replacement is necessary.
        /// </summary>
        public bool? DeleteBeforeReplace { get; set; }

        private List<string>? _additionalSecretOutputs;

        /// <summary>
        /// The names of outputs for this resource that should be treated as secrets. This augments
        /// the list that the resource provider and pulumi engine already determine based on inputs
        /// to your resource. It can be used to mark certain outputs as a secrets on a per resource
        /// basis.
        /// </summary>
        public List<string> AdditionalSecretOutputs
        {
            get => _additionalSecretOutputs ??= new List<string>();
            set => _additionalSecretOutputs = value;
        }

        /// <summary>
        /// When provided with a resource ID, import indicates that this resource's provider should
        /// import its state from the cloud resource with the given ID.The inputs to the resource's
        /// constructor must align with the resource's current state.Once a resource has been
        /// imported, the import property must be removed from the resource's options.
        /// </summary>
        public string? ImportId { get; set; }

        internal override ResourceOptions Clone()
            => CreateCustomResourceOptionsCopy(this);

        /// <summary>
        /// Takes two <see cref="CustomResourceOptions"/> values and produces a new
        /// <see cref="CustomResourceOptions"/> with the respective
        /// properties of <paramref name="options2"/> merged over the same properties in <paramref
        /// name="options1"/>. The original options objects will be unchanged.
        /// <para/>
        /// A new instance will always be returned.
        /// <para/>
        /// Conceptually property merging follows these basic rules:
        /// <list type="number">
        /// <item>
        /// If the property is a collection, the final value will be a collection containing the
        /// values from each options object.
        /// </item>
        /// <item>
        /// Simple scalar values from <paramref name="options2"/> (i.e. <see cref="string"/>s,
        /// <see cref="int"/>s, <see cref="bool"/>s) will replace the values of <paramref
        /// name="options1"/>.
        /// </item>
        /// <item>
        /// <see langword="null"/> values in <paramref name="options2"/> will be ignored.
        /// </item>
        /// </list>
        /// </summary>
        public static CustomResourceOptions Merge(CustomResourceOptions? options1, CustomResourceOptions? options2)
        {
            options1 = options1 != null ? CreateCustomResourceOptionsCopy(options1) : new CustomResourceOptions();
            options2 = options2 != null ? CreateCustomResourceOptionsCopy(options2) : new CustomResourceOptions();

            // first, merge all the normal option values over
            MergeNormalOptions(options1, options2);

            options1.DeleteBeforeReplace = options2.DeleteBeforeReplace ?? options1.DeleteBeforeReplace;
            options1.ImportId = options2.ImportId ?? options1.ImportId;

            options1.AdditionalSecretOutputs.AddRange(options2.AdditionalSecretOutputs);
            return options1;
        }
    }
}
