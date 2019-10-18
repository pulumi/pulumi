// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;

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
                ImmutableDictionary<string, Input<object>> properties,
                ResourceOptions? opts = null)
            : base($"pulumi:providers:${package}", name, properties, opts)
        {
            this.Package = package;
        }
    }
}
