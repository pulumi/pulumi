// Copyright 2016-2018, Pulumi Corporation

using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
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
                CompleteResourceAsync(resource, () => RegisterResourceAsync(resource, custom, args, options)));
        }

        private async Task<(string urn, string id, Struct data)> RegisterResourceAsync(
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

            request.Aliases.AddRange(prepareResult.Aliases);
            request.Dependencies.AddRange(prepareResult.AllDirectDependencyURNs);

            foreach (var (key, resourceURNs) in prepareResult.PropertyToDirectDependencyURNs)
            {
                var deps = new RegisterResourceRequest.Types.PropertyDependencies();
                deps.Urns.AddRange(resourceURNs);
                request.PropertyDependencies.Add(key, deps);
            }

            request.Object = prepareResult.SerializedProps;
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
