// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private Task<Urn>? _rootResource;

        /// <summary>
        /// returns a root resource URN that will automatically become the default parent of all
        /// resources.  This can be used to ensure that all resources without explicit parents are
        /// parented to a common parent resource.
        /// </summary>
        /// <returns></returns>
        internal async Task<Urn?> GetRootResourceAsync(string type)
        {
            // If we're calling this while creating the stack itself.  No way to know its urn at
            // this point.
            if (type == Stack._rootPulumiStackTypeName)
                return null;

            if (_rootResource == null)
                throw new InvalidOperationException($"Calling {nameof(GetRootResourceAsync)} before the root resource was registered!");

            return await _rootResource.ConfigureAwait(false);
        }

        internal Task SetRootResourceAsync(Stack stack)
        {
            if (_rootResource != null)
                throw new InvalidOperationException("Tried to set the root resource more than once!");

            _rootResource = SetRootResourceWorkerAsync(stack);
            return _rootResource;
        }

        internal async Task<Urn> SetRootResourceWorkerAsync(Stack stack)
        {
            var resUrn = await stack.Urn.GetValueAsync().ConfigureAwait(false);
            await this.Engine.SetRootResourceAsync(new SetRootResourceRequest
            {
                Urn = resUrn.Value,
            });

            var getResponse = await this.Engine.GetRootResourceAsync(new GetRootResourceRequest());
            return new Urn(getResponse.Urn);
        }
    }
}
