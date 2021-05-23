// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Threading.Tasks;
using Google.Protobuf;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        void IDeploymentInternal.RegisterResourceOutputs(Resource resource, Output<IDictionary<string, object?>> outputs)
        {
            // RegisterResourceOutputs is called in a fire-and-forget manner.  Make sure we keep track of
            // this task so that the application will not quit until this async work completes.
            _runner.RegisterTask(
                $"{nameof(IDeploymentInternal.RegisterResourceOutputs)}: {resource.GetResourceType()}-{resource.GetResourceName()}",
                RegisterResourceOutputsAsync(resource, outputs));
        }

        private async Task RegisterResourceOutputsAsync(
            Resource resource, Output<IDictionary<string, object?>> outputs)
        {
            var opLabel = "monitor.registerResourceOutputs(...)";

            // The registration could very well still be taking place, so we will need to wait for its URN.
            // Additionally, the output properties might have come from other resources, so we must await those too.
            var urn = await resource.Urn.GetValueAsync().ConfigureAwait(false);
            var props = await outputs.GetValueAsync().ConfigureAwait(false);

            var serialized = await SerializeAllPropertiesAsync(
                opLabel, props, await MonitorSupportsResourceReferences().ConfigureAwait(false)).ConfigureAwait(false);
            Log.Debug($"RegisterResourceOutputs RPC prepared: urn={urn}" +
                (_excessiveDebugOutput ? $", outputs={JsonFormatter.Default.Format(serialized)}" : ""));

            await Monitor.RegisterResourceOutputsAsync(new RegisterResourceOutputsRequest
            {
                Urn = urn,
                Outputs = serialized,
            });
        }
    }
}
