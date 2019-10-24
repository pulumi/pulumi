// Copyright 2016-2018, Pulumi Corporation

using System;
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
            request.Object = prepareResult.SerializedProps;

            request.Parent = prepareResult.ParentUrn ?? "";
            request.Provider = prepareResult.ProviderRef ?? "";

            request.Aliases.AddRange(prepareResult.Aliases);
            request.Dependencies.AddRange(prepareResult.AllDirectDependencyURNs);

            foreach (var (key, resourceURNs) in prepareResult.PropertyToDirectDependencyURNs)
            {
                var deps = new RegisterResourceRequest.Types.PropertyDependencies();
                deps.Urns.AddRange(resourceURNs);
                request.PropertyDependencies.Add(key, deps);
            }
        }

        private static RegisterResourceRequest CreateRegisterResourceRequest(string type, string name, bool custom, ResourceOptions options)
        {
            var customOpts = options as CustomResourceOptions;
            var deleteBeforeReplace = customOpts?.DeleteBeforeReplace;

            var request = new RegisterResourceRequest()
            {
                Type = type,
                Name = name,
                Custom = custom,
                Protect = options.Protect ?? false,
                Version = options.Version ?? "",
                ImportId = customOpts?.ImportId ?? "",
                AcceptSecrets = true,
                DeleteBeforeReplace = deleteBeforeReplace ?? false,
                DeleteBeforeReplaceDefined = deleteBeforeReplace != null,
                CustomTimeouts = new RegisterResourceRequest.Types.CustomTimeouts
                {
                    Create = TimeoutString(options.CustomTimeouts?.Create),
                    Delete = TimeoutString(options.CustomTimeouts?.Delete),
                    Update = TimeoutString(options.CustomTimeouts?.Update),
                },
            };

            if (customOpts != null)
                request.AdditionalSecretOutputs.AddRange(customOpts.AdditionalSecretOutputs);

            request.IgnoreChanges.AddRange(options.IgnoreChanges);

            return request;
        }

        private static string TimeoutString(TimeSpan? timeSpan)
        {
            if (timeSpan == null)
                return "";

            // This will eventually be parsed by go's ParseDuration function here:
            // https://github.com/pulumi/pulumi/blob/06d4dde8898b2a0de2c3c7ff8e45f97495b89d82/pkg/resource/deploy/source_eval.go#L967
            //
            // So we generate a legal duration as allowed by
            // https://golang.org/pkg/time/#ParseDuration.
            //
            // Simply put, we simply convert our ticks to the integral number of nanoseconds
            // corresponding to it.  Since each tick is 100ns, this can trivialy be done just by
            // appending "00" to it.
            return timeSpan.Value.Ticks.ToString() + "00ns";
        }
    }
}
