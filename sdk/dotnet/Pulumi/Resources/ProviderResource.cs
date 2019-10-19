// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Rpc;

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

        //    /** @internal */
        //    // tslint:disable-next-line: variable-name
        //    public __registrationId?: string;
        private string? _registrationId;

        //    public static async register(provider: ProviderResource | undefined): Promise<string | undefined> {
        //        if (provider === undefined) {
        //            return undefined;
        //        }

        //        if (!provider.__registrationId) {
        //            const providerURN = await provider.urn.promise();
        //    const providerID = await provider.id.promise() || unknownValue;
        //    provider.__registrationId = `${providerURN
        //}::${providerID}`;
        //        }

        //        return provider.__registrationId;
        //    }

        /// <summary>
        /// Creates and registers a new provider resource for a particular package.
        /// </summary>
        public ProviderResource(
            string package, string name,
            ResourceArgs args, ResourceOptions? opts = null)
            : base($"pulumi:providers:${package}", name, args, opts)
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
                var providerIDVal = providerID.Value;
                if (string.IsNullOrEmpty(providerIDVal))
                {
                    providerIDVal = Constants.UnknownValue;
                }

                provider._registrationId = $"{providerURN}::{providerIDVal}";
            }

            return provider._registrationId;
        }
    }
}
