// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumirpc;

namespace Pulumi
{
    public partial class Deployment
    {
        private async Task<(string urn, string id, Struct data, ImmutableDictionary<string, ImmutableHashSet<Resource>> dependencies)> RegisterResourceAsync(
            Resource resource, bool remote, Func<string, Resource> newDependency, ResourceArgs args,
            ResourceOptions options)
        {
            var name = resource.GetResourceName();
            var type = resource.GetResourceType();
            var custom = resource is CustomResource;

            var label = $"resource:{name}[{type}]";
            Log.Debug($"Registering resource start: t={type}, name={name}, custom={custom}, remote={remote}");

            var request = CreateRegisterResourceRequest(type, name, custom, remote, options);

            Log.Debug($"Preparing resource: t={type}, name={name}, custom={custom}, remote={remote}");
            var prepareResult = await PrepareResourceAsync(label, resource, custom, remote, args, options).ConfigureAwait(false);
            Log.Debug($"Prepared resource: t={type}, name={name}, custom={custom}, remote={remote}");

            PopulateRequest(request, prepareResult);

            Log.Debug($"Registering resource monitor start: t={type}, name={name}, custom={custom}, remote={remote}");
            var result = await this.Monitor.RegisterResourceAsync(resource, request);
            Log.Debug($"Registering resource monitor end: t={type}, name={name}, custom={custom}, remote={remote}");

            var dependencies = ImmutableDictionary.CreateBuilder<string, ImmutableHashSet<Resource>>();
            foreach (var (key, propertyDependencies) in result.PropertyDependencies)
            {
                var urns = ImmutableHashSet.CreateBuilder<Resource>();
                foreach (var urn in propertyDependencies.Urns)
                {
                    urns.Add(newDependency(urn));
                }
                dependencies[key] = urns.ToImmutable();
            }

            return (result.Urn, result.Id, result.Object, dependencies.ToImmutable());
        }

        private static void PopulateRequest(RegisterResourceRequest request, PrepareResult prepareResult)
        {
            request.Object = prepareResult.SerializedProps;
            request.Parent = prepareResult.ParentUrn;
            request.Provider = prepareResult.ProviderRef;
            request.Providers.Add(prepareResult.ProviderRefs);
            request.Aliases.AddRange(prepareResult.Aliases);
            request.Dependencies.AddRange(prepareResult.AllDirectDependencyUrns);

            foreach (var (key, resourceUrns) in prepareResult.PropertyToDirectDependencyUrns)
            {
                var deps = new RegisterResourceRequest.Types.PropertyDependencies();
                deps.Urns.AddRange(resourceUrns);
                request.PropertyDependencies.Add(key, deps);
            }
        }

        private static RegisterResourceRequest CreateRegisterResourceRequest(
            string type, string name, bool custom, bool remote, ResourceOptions options)
        {
            var customOpts = options as CustomResourceOptions;
            var deleteBeforeReplace = customOpts?.DeleteBeforeReplace;

            var request = new RegisterResourceRequest
            {
                Type = type,
                Name = name,
                Custom = custom,
                Protect = options.Protect ?? false,
                Version = options.Version ?? "",
                ImportId = customOpts?.ImportId ?? "",
                AcceptSecrets = true,
                AcceptResources = !_disableResourceReferences,
                DeleteBeforeReplace = deleteBeforeReplace ?? false,
                DeleteBeforeReplaceDefined = deleteBeforeReplace != null,
                CustomTimeouts = new RegisterResourceRequest.Types.CustomTimeouts
                {
                    Create = TimeoutString(options.CustomTimeouts?.Create),
                    Delete = TimeoutString(options.CustomTimeouts?.Delete),
                    Update = TimeoutString(options.CustomTimeouts?.Update),
                },
                Remote = remote,
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
            return timeSpan.Value.Ticks + "00ns";
        }
    }
}
