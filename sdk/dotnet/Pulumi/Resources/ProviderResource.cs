// Copyright 2016-2019, Pulumi Corporation

using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// <see cref="ProviderResource"/> is a <see cref="Resource"/> that implements CRUD operations
    /// for other custom resources. These resources are managed similarly to other resources,
    /// including the usual diffing and update semantics.
    /// </summary>
    public class ProviderResource : CustomResource
    {
        internal string Package { get; }

        private string? _registrationId;

        /// <summary>
        /// Creates and registers a new provider resource for a particular package.
        /// </summary>
        /// <param name="package">The package associated with this provider.</param>
        /// <param name="name">The unique name of the provider.</param>
        /// <param name="args">The configuration to use for this provider.</param>
        /// <param name="options">A bag of options that control this provider's behavior.</param>
        public ProviderResource(string package, string name, ResourceArgs args, CustomResourceOptions? options = null)
            : this(package, name, args, options, dependency: false)
        {
        }

        /// <summary>
        /// Creates and registers a new provider resource for a particular package.
        /// </summary>
        /// <param name="package">The package associated with this provider.</param>
        /// <param name="name">The unique name of the provider.</param>
        /// <param name="args">The configuration to use for this provider.</param>
        /// <param name="options">A bag of options that control this provider's behavior.</param>
        /// <param name="dependency">True if this is a synthetic resource used internally for dependency tracking.</param>
        private protected ProviderResource(
            string package, string name,
            ResourceArgs args, CustomResourceOptions? options = null, bool dependency = false)
            : base($"pulumi:providers:{package}", name, args, options, dependency)
        {
            this.Package = package;
        }

        internal static async Task<string?> RegisterAsync(ProviderResource? provider)
        {
            if (provider == null)
            {
                return null;
            }

            if (provider._registrationId == null)
            {
                var providerUrn = await provider.Urn.GetValueAsync().ConfigureAwait(false);
                var providerId = await provider.Id.GetValueAsync().ConfigureAwait(false);
                if (string.IsNullOrEmpty(providerId))
                {
                    providerId = Constants.UnknownValue;
                }

                provider._registrationId = $"{providerUrn}::{providerId}";
            }

            return provider._registrationId;
        }
    }
}
