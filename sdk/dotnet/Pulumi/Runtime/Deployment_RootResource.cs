// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Threading.Tasks;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private Task<string>? _rootResource;

        /// <summary>
        /// returns a root resource URN that will automatically become the default parent of all
        /// resources.  This can be used to ensure that all resources without explicit parents are
        /// parented to a common parent resource.
        /// </summary>
        /// <returns></returns>
        internal async Task<string?> GetRootResourceAsync(string type)
        {
            // If we're calling this while creating the stack itself.  No way to know its urn at
            // this point.
            if (type == Stack._rootPulumiStackTypeName)
                return null;

            if (_rootResource == null)
                throw new InvalidOperationException($"Calling {nameof(GetRootResourceAsync)} before the root resource was registered!");

            return await _rootResource.ConfigureAwait(false);
        }

        Task IDeploymentInternal.SetRootResourceAsync(Stack stack)
        {
            if (_rootResource != null)
                throw new InvalidOperationException("Tried to set the root resource more than once!");

            _rootResource = SetRootResourceWorkerAsync(stack);
            return _rootResource;
        }

        private async Task<string> SetRootResourceWorkerAsync(Stack stack)
        {
            var resUrn = await stack.Urn.GetValueAsync().ConfigureAwait(false);
            await this.Engine.SetRootResourceAsync(new SetRootResourceRequest
            {
                Urn = resUrn,
            });

            var getResponse = await this.Engine.GetRootResourceAsync(new GetRootResourceRequest());
            return getResponse.Urn;
        }
    }
}
