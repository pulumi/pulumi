// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Newtonsoft.Json.Linq;
using Pulumi.Rpc;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        void IDeploymentInternal.RegisterResource(
            Resource resource, bool custom, ResourceArgs args, ResourceOptions options)
        {
            // RegisterResource is called in a fire-and-forget manner.  Make sure we keep track of
            // this task so that the application will not quit until this async work completes.
            //
            // Also, we can only do our work once the constructor for the resource has actually
            // finished.  Otherwise, we might actually register and get the result back *prior* to
            // the object finishing initializing.  Note: this is not a speculative concern. This is
            // something that does happen and has to be accounted for.
            this.RegisterTask(
                $"{nameof(IDeploymentInternal.RegisterResource)}: {resource.GetResourceType()}-{resource.GetResourceName()}",
                resource._onConstructorFinished.Task.ContinueWith(
                    _ => RegisterResourceAsync(resource, custom, args, options),
                    CancellationToken.None, TaskContinuationOptions.None, TaskScheduler.Default).Unwrap());
        }

        private static ImmutableDictionary<string, IOutputCompletionSource> GetOutputCompletionSources(
            Resource resource)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();

            var query = from field in resource.GetType().GetFields(BindingFlags.NonPublic | BindingFlags.Instance)
                        let attr = field.GetCustomAttribute<ResourceFieldAttribute>()
                        where attr != null
                        select (field, attr);

            var result = ImmutableDictionary.CreateBuilder<string, IOutputCompletionSource>();
            foreach (var (field, attr) in query.ToList())
            {
                var completionSource = (IOutputCompletionSource?)field.GetValue(resource);
                if (completionSource == null)
                {
                    throw new InvalidOperationException("[ResourceField] attribute was placed on a null field.");
                }

                result.Add(attr.Name, completionSource);
            }

            result.Add("urn", resource._urn);
            if (resource is CustomResource customResource)
                result.Add("id", customResource._id);
            
            Log.Debug("Fields to assign: " + new JArray(result.Keys), resource);
            return result.ToImmutable();
        }

        private Task RegisterResourceAsync(Resource resource, bool custom, ResourceArgs args, ResourceOptions options)
            => CompleteResourceAsync(
                resource, () => RegisterResourceWorkerAsync(resource, custom, args, options));

        private async Task<(string urn, string id, Struct data)> RegisterResourceWorkerAsync(
            Resource resource, bool custom,
            ResourceArgs args, ResourceOptions options)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();

            var label = $"resource:{name}[{type}]";
            Log.Debug($"Registering resource start: t={type}, name={name}, custom={custom}");

            var request = CreateRegisterResourceRequest(type, name, custom, options);

            Log.Debug($"Preparing resource: t={type}, name={name}, custom={custom}");
            var prepareResult = await PrepareResourceAsync(label, resource, custom, args, options).ConfigureAwait(false);
            Log.Debug($"Prepared resource: t={type}, name={name}, custom={custom}");

            PopulateRequest(request, prepareResult);

            Log.Debug($"Registering resource monitor start: t={type}, name={name}, custom={custom}");
            var result = await this.Monitor.RegisterResourceAsync(request);
            Log.Debug($"Registering resource monitor end: t={type}, name={name}, custom={custom}");
            return (result.Urn, result.Id, result.Object);
        }

        private static void PopulateRequest(RegisterResourceRequest request, PrepareResult prepareResult)
        {
            if (prepareResult.ParentUrn != null)
                request.Parent = prepareResult.ParentUrn;

            if (prepareResult.ProviderRef != null)
                request.Provider = prepareResult.ProviderRef;

            foreach (var alias in prepareResult.Aliases)
                request.Aliases.Add(alias);

            foreach (var dep in prepareResult.AllDirectDependencyURNs)
                request.Dependencies.Add(dep);

            foreach (var (key, resourceURNs) in prepareResult.PropertyToDirectDependencyURNs)
            {
                var deps = new RegisterResourceRequest.Types.PropertyDependencies();
                deps.Urns.AddRange(resourceURNs);
                request.PropertyDependencies.Add(key, deps);
            }

            request.Object = prepareResult.SerializedProps;
        }

        private static Value CreateValue(object? value)
            => value switch
            {
                null => Value.ForNull(),
                int i => Value.ForNumber(i),
                double d => Value.ForNumber(d),
                bool b => Value.ForBool(b),
                string s => Value.ForString(s),
                IList list => Value.ForList(list.OfType<object>().Select(v => CreateValue(v)).ToArray()),
                IDictionary dict => Value.ForStruct(CreateStruct(dict)),
                _ => throw new InvalidOperationException("Unsupported value when converting to protobuf: " + value.GetType().FullName),
            };

        private static Struct CreateStruct(IDictionary dict)
        {
            var result = new Struct();
            foreach (var key in dict.Keys.OfType<string>())
            {
                result.Fields.Add(key, CreateValue(dict[key]));
            }
            return result;
        }

        private static RegisterResourceRequest CreateRegisterResourceRequest(string type, string name, bool custom, ResourceOptions options)
        {
            var customOpts = options as CustomResourceOptions;
            var deleteBeforeReplace = customOpts?.DeleteBeforeReplace;
            var importID = customOpts?.ImportId;

            var request = new RegisterResourceRequest()
            {
                Type = type,
                Name = name,
                Custom = custom,
                Protect = options.Protect ?? false,
                Version = options.Version ?? "",
                ImportId = importID ?? "",
                AcceptSecrets = true,

                CustomTimeouts = new RegisterResourceRequest.Types.CustomTimeouts(),
                DeleteBeforeReplace = deleteBeforeReplace ?? false,
                DeleteBeforeReplaceDefined = deleteBeforeReplace != null,
            };

            if (customOpts != null)
                request.AdditionalSecretOutputs.AddRange(customOpts.AdditionalSecretOutputs);

            request.IgnoreChanges.AddRange(options.IgnoreChanges);

            if (options.CustomTimeouts?.Create != null)
                request.CustomTimeouts.Create = options.CustomTimeouts.Create;

            if (options.CustomTimeouts?.Delete != null)
                request.CustomTimeouts.Delete = options.CustomTimeouts.Delete;

            if (options.CustomTimeouts?.Update != null)
                request.CustomTimeouts.Update = options.CustomTimeouts.Update;

            return request;
        }
    }
}
