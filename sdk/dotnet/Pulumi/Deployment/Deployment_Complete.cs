// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// Executes the provided <paramref name="action"/> and then completes all the 
        /// <see cref="OutputCompletionSource{T}"/> sources on the <paramref name="resource"/> with
        /// the results of it.
        /// </summary>
        private async Task CompleteResourceAsync(
            Resource resource, Func<Task<(string urn, string id, Struct data)>> action)
        {
            var completionSources = GetOutputCompletionSources(resource);

            // Run in a try/catch/finally so that we always resolve all the outputs of the resource
            // regardless of whether we encounter an errors computing the action.
            try
            {
                var response = await action().ConfigureAwait(false);
                
                resource._urn.SetResult(response.urn);
                if (resource is CustomResource customResource)
                {
                    var id = response.id;
                    if (string.IsNullOrEmpty(id))
                    {
                        customResource._id.SetUnknownResult();
                    }
                    else
                    {
                        customResource._id.SetResult(id);
                    }
                }

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    if (completionSource is IProtobufOutputCompletionSource pbCompletionSource &&
                        response.data.Fields.TryGetValue(fieldName, out var value))
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
                    source.SetDefaultResult(isKnown: !this.IsDryRun);
                }
            }
        }
    }
}