// Copyright 2016-2020, Pulumi Corporation

namespace Pulumi.Kubernetes
{
    /// <summary>
    /// A base class for all Kubernetes resources.
    /// </summary>
    public abstract class KubernetesResource : CustomResource
    {
        /// <summary>
        /// Standard constructor passing arguments to <see cref="CustomResource"/>.
        /// </summary>
        internal KubernetesResource(string type, string name, ResourceArgs? args, CustomResourceOptions? options = null)
            : base(type, name, args, options)
        {
        }

        /// <summary>
        /// Additional constructor for dynamic arguments received from YAML-based sources.
        /// </summary>
        internal KubernetesResource(string type, string name, DictionaryResourceArgs? args, CustomResourceOptions? options = null)
            : base(type, name, args, options)
        {
        }
    }
}
