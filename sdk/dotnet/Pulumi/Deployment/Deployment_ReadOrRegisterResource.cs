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
        void IDeploymentInternal.ReadOrRegisterResource(
            Resource resource, bool remote, Func<string, Resource> newDependency, ResourceArgs args,
            ResourceOptions options)
        {
            // ReadOrRegisterResource is called in a fire-and-forget manner.  Make sure we keep
            // track of this task so that the application will not quit until this async work
            // completes.
            //
            // Also, we can only do our work once the constructor for the resource has actually
            // finished.  Otherwise, we might actually read and get the result back *prior* to the
            // object finishing initializing.  Note: this is not a speculative concern. This is
            // something that does happen and has to be accounted for.
            //
            // IMPORTANT! We have to make sure we run 'OutputCompletionSource.InitializeOutputs'
            // synchronously directly when `resource`'s constructor runs since this will set all of
            // the `[Output(...)] Output<T>` properties.  We need those properties assigned by the
            // time the base 'Resource' constructor finishes so that both derived classes and
            // external consumers can use the Output properties of `resource`.
            var completionSources = OutputCompletionSource.InitializeOutputs(resource);

            _runner.RegisterTask(
                $"{nameof(IDeploymentInternal.ReadOrRegisterResource)}: {resource.GetResourceType()}-{resource.GetResourceName()}",
                CompleteResourceAsync(resource, remote, newDependency, args, options, completionSources));
        }

        private async Task<(string urn, string id, Struct data, ImmutableDictionary<string, ImmutableHashSet<Resource>> dependencies)> ReadOrRegisterResourceAsync(
            Resource resource, bool remote, Func<string, Resource> newDependency, ResourceArgs args,
            ResourceOptions options)
        {
            if (options.Urn != null)
            {
                // This is a resource that already exists. Read its state from the engine.
                var result = await InvokeRawAsync(
                    "pulumi:pulumi:getResource",
                    new GetResourceInvokeArgs {Urn = options.Urn},
                    new InvokeOptions());
                
                var urn = result.Fields["urn"].StringValue;
                var id = result.Fields["id"].StringValue;
                var state = result.Fields["state"].StructValue;
                return (urn, id, state, ImmutableDictionary<string, ImmutableHashSet<Resource>>.Empty);
            }
            
            if (options.Id != null)
            {
                var id = await options.Id.ToOutput().GetValueAsync().ConfigureAwait(false);
                if (!string.IsNullOrEmpty(id))
                {
                    if (!(resource is CustomResource))
                        throw new ArgumentException($"{nameof(ResourceOptions)}.{nameof(ResourceOptions.Id)} is only valid for a {nameof(CustomResource)}");

                    // If this resource already exists, read its state rather than registering it anew.
                    return await ReadResourceAsync(resource, id, args, options).ConfigureAwait(false);
                }
            }

            // Kick off the resource registration.  If we are actually performing a deployment, this
            // resource's properties will be resolved asynchronously after the operation completes,
            // so that dependent computations resolve normally.  If we are just planning, on the
            // other hand, values will never resolve.
            return await RegisterResourceAsync(resource, remote, newDependency, args, options).ConfigureAwait(false);
        }

        /// <summary>
        /// Calls <see cref="ReadOrRegisterResourceAsync"/> then completes all the 
        /// <see cref="IOutputCompletionSource"/> sources on the <paramref name="resource"/> with
        /// the results of it.
        /// </summary>
        private async Task CompleteResourceAsync(
            Resource resource, bool remote, Func<string, Resource> newDependency, ResourceArgs args,
            ResourceOptions options, ImmutableDictionary<string, IOutputCompletionSource> completionSources)
        {
            // Run in a try/catch/finally so that we always resolve all the outputs of the resource
            // regardless of whether we encounter an errors computing the action.
            try
            {
                var response = await ReadOrRegisterResourceAsync(
                    resource, remote, newDependency, args, options).ConfigureAwait(false);

                completionSources[Constants.UrnPropertyName].SetStringValue(response.urn, isKnown: true);
                if (resource is CustomResource)
                {
                    // ReSharper disable once ConstantNullCoalescingCondition
                    var id = response.id ?? "";
                    completionSources[Constants.IdPropertyName].SetStringValue(id, isKnown: id != "");
                }

                // Go through all our output fields and lookup a corresponding value in the response
                // object.  Allow the output field to deserialize the response.
                foreach (var (fieldName, completionSource) in completionSources)
                {
                    if (fieldName == Constants.UrnPropertyName || fieldName == Constants.IdPropertyName)
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
                        if (!response.dependencies.TryGetValue(fieldName, out var dependencies))
                        {
                            dependencies = ImmutableHashSet<Resource>.Empty;
                        }

                        var converted = Converter.ConvertValue($"{resource.GetType().FullName}.{fieldName}", value,
                            completionSource.TargetType, dependencies);
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
                    source.TrySetDefaultResult(isKnown: !_isDryRun);
                }
            }
        }

        // Arguments type for the `getResource` invoke.
        private class GetResourceInvokeArgs : InvokeArgs
        {
            [Input("urn", required: true)]
            public string Urn { get; set; } = null!;
        }
    }
}
