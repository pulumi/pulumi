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
            try
            {
                var response = await RegisterResourceWorkerAsync(
                    resource, type, name, custom, args, opts).ConfigureAwait(false);

                resource._urn.SetResult(response.Urn);
                if (resource is CustomResource customResource)
                    customResource._id.SetResult(response.Id);

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    if (completionSource is IProtobufOutputCompletionSource pbCompletionSource &&
                        response.Object.Fields.TryGetValue(fieldName, out var value))
                    {
                        pbCompletionSource.SetResult(value);
                    }
                }
            }
            catch (Exception e)
            {
                // Mark any unresolved output properties with this exception.  That way we don't
                // leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    source.TrySetException(e);
                }
            }
            finally
            {
                // ensure that we've at least resolved all our completion sources.  That way we
                // don't leave any outstanding tasks sitting around which might cause hangs.
                foreach (var source in completionSources.Values)
                {
                    // Didn't get a value for this field.  Resolve it with a default value.
                    // If we're in preview, we'll consider this unknown and in a normal
                    // update we'll consider it known.
                    source.SetDefaultResult(isKnown: !this.Options.DryRun);
                }
            }
        }
    }
}
