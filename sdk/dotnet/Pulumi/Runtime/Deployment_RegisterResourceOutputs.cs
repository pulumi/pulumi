// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;
using Pulumirpc;

using System.Collections.Generic;

namespace Pulumi
{
    public partial class Deployment
    {
        internal void RegisterResourceOutputs(
            Resource resource, Output<IDictionary<string, object>> outputs)
        {
            var task1 = RegisterResourceOutputsAsync(resource, outputs);
            // RegisterResourceOutputs is called in a fire-and-forget manner.  Make sure we keep track of
            // this task so that the application will not quit until this async work completes.
            this.RegisterTask(
                $"{nameof(RegisterResourceOutputs)}: {resource.Type}-{resource.Name}",
                task1);
        }

        private async Task RegisterResourceOutputsAsync(
            Resource resource, Output<IDictionary<string, object>> outputs)
        {
            var opLabel = $"monitor.registerResourceOutputs(...)";

            // The registration could very well still be taking place, so we will need to wait for its URN.
            // Additionally, the output properties might have come from other resources, so we must await those too.
            var urn = await resource.Urn.GetValueAsync().ConfigureAwait(false);
            var props = await outputs.GetValueAsync().ConfigureAwait(false);
            var propInputs = props.ToDictionary(kvp => kvp.Key, kvp => (IInput)kvp.ToObjectOutput());

            var serialized = await SerializeAllPropertiesAsync(opLabel, propInputs).ConfigureAwait(false);
            Log.Debug(`RegisterResourceOutputs RPC prepared: urn =${ urn}` +
(excessiveDebugOutput ? `, outputs =${ JSON.stringify(outputsObj)}` : ``));

            // Fetch the monitor and make an RPC request.
            const monitor = getMonitor();
            if (monitor)
            {
                const req = new resproto.RegisterResourceOutputsRequest();
                req.setUrn(urn);
                req.setOutputs(outputsObj);

                const label = `monitor.registerResourceOutputs(${ urn}, ...)`;
                await debuggablePromise(new Promise((resolve, reject) =>
                    (monitor as any).registerResourceOutputs(req, (err: grpc.ServiceError, innerResponse: any) => {
                    log.debug(`RegisterResourceOutputs RPC finished: urn=${urn}; ` +
                            `err: ${ err}, resp: ${ innerResponse}`);
                if (err)
                {
                    // If the monitor is unavailable, it is in the process of shutting down or has already
                    // shut down. Don't emit an error and don't do any more RPCs, just exit.
                    if (err.code === grpc.status.UNAVAILABLE)
                    {
                        log.debug("Resource monitor is terminating");
                        process.exit(0);
                    }

                    log.error(`Failed to end new resource registration '${urn}': ${ err.stack}`);
                    reject(err);
                }
                else
                {
                    resolve();
                }
            })), label);

        }
    }
}
