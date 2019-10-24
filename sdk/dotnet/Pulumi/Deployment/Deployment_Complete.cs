// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;

namespace Pulumi
{
    public partial class Deployment
    {
        /// <summary>
        /// Executes the provided <paramref name="action"/> and then completes all the 
        /// <see cref="IOutputCompletionSource"/> sources on the <paramref name="resource"/> with
        /// the results of it.
        /// </summary>
        private Task CompleteResourceAsync(
            Resource resource, Func<Task<(string urn, string id, Struct data)>> action)
        {
            // IMPORTANT!  This function must not be `async`.  We have to make sure this
            // synchronously happens. When the constructor runs since this will set all
            // our output fields.

            return CompleteResourceAsync(
                resource, action, OutputCompletionSource.GetSources(resource));
        }

        private async Task CompleteResourceAsync(
            Resource resource, Func<Task<(string urn, string id, Struct data)>> action,
            ImmutableDictionary<string, IOutputCompletionSource> completionSources)
        {
            // Run in a try/catch/finally so that we always resolve all the outputs of the resource
            // regardless of whether we encounter an errors computing the action.
            try
            {
                var response = await action().ConfigureAwait(false);
                completionSources["urn"].SetStringValue(response.urn, isKnown: true);
                if (resource is CustomResource customResource)
                {
                    var id = response.id;
                    if (string.IsNullOrEmpty(id))
                    {
                        completionSources["id"].SetStringValue("", isKnown: false);
                    }
                    else
                    {
                        completionSources["id"].SetStringValue(id, isKnown: true);
                    }
                }

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    if (fieldName == "urn" || fieldName == "id")
                    {
                        // Already handled specially above.
                        continue;
                    }

                    // We process and deserialize each field (instead of bulk processing
                    // 'response.data' so that each field can have independent isKnown/isSecret
                    // values.  We do not want to bubble up isKnown/isSecret from one field to the
                    // rest.
                    if (response.data.Fields.TryGetValue(fieldName, out var value))
                    {
                        var converted = Converter.ConvertValue(
                            $"{resource.GetType().FullName}.{fieldName}", value, completionSource.TargetType);
                        completionSource.SetValue(converted);
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

                throw;
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
                    source.TrySetDefaultResult(isKnown: !this.IsDryRun);
                }
            }
        }
    }
}
