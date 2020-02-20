﻿// Copyright 2016-2019, Pulumi Corporation

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
        internal readonly string Package;

        private string? _registrationId;

        /// <summary>
        /// Creates and registers a new provider resource for a particular package.
        /// </summary>
        public ProviderResource(
            string package, string name,
            ResourceArgs args, CustomResourceOptions? options = null)
            : base($"pulumi:providers:{package}", name, args, options)
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
                var providerURN = await provider.Urn.GetValueAsync().ConfigureAwait(false);
                var providerID = await provider.Id.GetValueAsync().ConfigureAwait(false);
                if (string.IsNullOrEmpty(providerID))
                {
                    providerID = Constants.UnknownValue;
                }

                provider._registrationId = $"{providerURN}::{providerID}";
            }

            return provider._registrationId;
        }
    }
}
