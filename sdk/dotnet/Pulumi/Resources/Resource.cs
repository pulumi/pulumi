// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Pulumi.Rpc;

namespace Pulumi
{
    /// <summary>
    /// Resource represents a class whose CRUD operations are implemented by a provider plugin.
    /// </summary>
    public class Resource
    {
        /// <summary>
        /// The optional parent of this resource.
        /// </summary>
        private readonly Resource? _parentResource;

        /// <summary>
        /// The child resources of this resource.  We use these (only from a ComponentResource) to
        /// allow code to dependOn a ComponentResource and have that effectively mean that it is
        /// depending on all the CustomResource children of that component.
        /// 
        /// Important!  We only walk through ComponentResources.They're the only resources that
        /// serve as an aggregation of other primitive(i.e.custom) resources.While a custom resource
        /// can be a parent of other resources, we don't want to ever depend on those child
        /// resource.  If we do, it's simple to end up in a situation where we end up depending on a
        /// child resource that has a data cycle dependency due to the data passed into it. An
        /// example of how this would be bad is:
        /// 
        /// <c>
        ///     var c1 = new CustomResource("c1");
        ///     var c2 = new CustomResource("c2", { parentId = c1.id }, { parent = c1 });
        ///     var c3 = new CustomResource("c3", { parentId = c1.id }, { parent = c1 });
        /// </c>
        /// 
        /// The problem here is that 'c2' has a data dependency on 'c1'.  If it tries to wait on
        /// 'c1' it will walk to the children and wait on them.This will mean it will wait on 'c3'.
        /// But 'c3' will be waiting in the same manner on 'c2', and a cycle forms. This normally
        /// does not happen with ComponentResources as they do not have any data flowing into
        /// them.The only way you would be able to have a problem is if you had this sort of coding
        /// pattern:
        /// 
        /// <c>
        ///     var c1 = new ComponentResource("c1");
        ///     var c2 = new CustomResource("c2", { parentId = c1.urn }, { parent: c1 });
        ///     var c3 = new CustomResource("c3", { parentId = c1.urn }, { parent: c1 });
        /// </c>
        /// 
        /// However, this would be pretty nonsensical as there is zero need for a custom resource to
        /// ever need to reference the urn of a component resource.  So it's acceptable if that sort
        /// of pattern failed in practice.
        /// </summary>
        internal readonly HashSet<Resource> ChildResources = new HashSet<Resource>();

        /// <summary>
        /// Urn is the stable logical URN used to distinctly address a resource, both before and
        /// after deployments.
        /// </summary>

        internal readonly UrnOutputCompletionSource _urn;
        public Output<Urn> Urn => _urn.Output;

        /// <summary>
        /// When set to true, protect ensures this resource cannot be deleted.
        /// </summary>
        private readonly bool _protect;

        /// <summary>
        /// A collection of transformations to apply as part of resource registration.
        /// </summary>
        private readonly ImmutableArray<ResourceTransformation> _transformations;

        /// <summary>
        /// A list of aliases applied to this resource.
        /// </summary>
        internal readonly ImmutableArray<Input<Urn>> _aliases;

        /// <summary>
        /// The type assigned to the resource at construction.
        /// </summary>
        internal readonly string Type;

        /// <summary>
        /// The name assigned to the resource at construction.
        /// </summary>
        internal readonly string Name;

        /// <summary>
        /// The set of providers to use for child resources. Keyed by package name (e.g. "aws").
        /// </summary>
        private readonly ImmutableDictionary<string, ProviderResource> _providers;

        internal readonly TaskCompletionSource<bool> _onConstructorFinished = new TaskCompletionSource<bool>();

        /// <summary>
        /// Creates and registers a new resource object.  <paramref name="type"/> is the fully
        /// qualified type token and <paramref name="name"/> is the "name" part to use in creating a
        /// stable and globally unique URN for the object. dependsOn is an optional list of other
        /// resources that this resource depends on, controlling the order in which we perform
        /// resource operations.
        /// </summary>
        /// <param name="type">The type of the resource.</param>
        /// <param name="name">The unique name of the resource.</param>
        /// <param name="custom">True to indicate that this is a custom resource, managed by a plugin.</param>
        /// <param name="args">The arguments to use to populate the new resource.</param>
        /// <param name="opts">A bag of options that control this resource's behavior.</param>
        private protected Resource(
            string type, string name, bool custom,
            ResourceArgs args, ResourceOptions opts)
        {
            if (string.IsNullOrEmpty(type))
                throw new ArgumentException(nameof(type));

            if (string.IsNullOrEmpty(name))
                throw new ArgumentException(nameof(name));

            // Before anything else - if there are transformations registered, invoke them in order
            // to transform the properties and options assigned to this resource.
            var parent = opts.Parent ?? Deployment.Instance._stack;
            if (parent == null && type != Stack._rootPulumiStackTypeName)
            {
                throw new InvalidOperationException("No stack instance, and we were not the stack itself.");
            }

            this.Type = type;
            this.Name = name;
            this._urn = new UrnOutputCompletionSource(this);

            var transformations = ImmutableArray.CreateBuilder<ResourceTransformation>();
            transformations.AddRange(opts.ResourceTransformations);
            if (parent != null)
            {
                transformations.AddRange(parent._transformations);
            }
            this._transformations = transformations.ToImmutable();

            foreach (var transformation in this._transformations)
            {
                var tres = transformation(this, type, name, args, opts);
                if (tres != null)
                {
                    if (tres.Value.opts.Parent != opts.Parent)
                    {
                        // This is currently not allowed because the parent tree is needed to
                        // establish what transformation to apply in the first place, and to compute
                        // inheritance of other resource options in the Resource constructor before
                        // transformations are run (so modifying it here would only even partially
                        // take affect).  It's theoretically possible this restriction could be
                        // lifted in the future, but for now just disallow re-parenting resources in
                        // transformations to be safe.
                        throw new ArgumentException("Transformations cannot currently be used to change the 'parent' of a resource.");
                    }

                    args = tres.Value.args;
                    opts = tres.Value.opts;
                }
            }

            // Make a shallow clone of opts to ensure we don't modify the value passed in.
            opts = opts.Clone();
            var componentOpts = opts as ComponentResourceOptions;
            var customOpts = opts as CustomResourceOptions;

            if (opts.Provider != null &&
                componentOpts?.Providers.Count > 0)
            {
                throw new ResourceException("Do not supply both 'provider' and 'providers' options to a ComponentResource.", opts.Parent);
            }

            // Check the parent type if one exists and fill in any default options.
            this._providers = ImmutableDictionary<string, ProviderResource>.Empty;

            if (opts.Parent != null)
            {
                this._parentResource = opts.Parent;
                this._parentResource.ChildResources.Add(this);

                if (opts.Protect == null)
                    opts.Protect = opts.Parent._protect;

                // Make a copy of the aliases array, and add to it any implicit aliases inherited from its parent
                opts.Aliases = opts.Aliases.ToList();
                foreach (var parentAlias in opts.Parent._aliases)
                {
                    opts.Aliases.Add(Pulumi.Urn.InheritedChildAlias(name, opts.Parent.Name, parentAlias, type));
                }

                this._providers = opts.Parent._providers;
            }

            if (custom)
            {
                var provider = customOpts?.Provider;
                if (provider == null)
                {
                    if (opts.Parent != null)
                    {
                        // If no provider was given, but we have a parent, then inherit the
                        // provider from our parent.

                        opts.Provider = opts.Parent.GetProvider(type);
                    }
                }
                else
                {
                    // If a provider was specified, add it to the providers map under this type's package so that
                    // any children of this resource inherit its provider.
                    var typeComponents = type.Split(":");
                    if (typeComponents.Length == 3)
                    {
                        var pkg = typeComponents[0];
                        this._providers = this._providers.Add(pkg, provider);
                    }
                }
            }
            else
            {
                // Note: we checked above that at most one of opts.provider or opts.providers is
                // set.

                // If opts.provider is set, treat that as if we were given a array of provider with
                // that single value in it.  Otherwise, take the array of providers, convert it to a
                // map and combine with any providers we've already set from our parent.
                var providerList = opts.Provider != null
                    ? new List<ProviderResource> { opts.Provider }
                    : componentOpts?.Providers;

                this._providers = this._providers.AddRange(ConvertToProvidersMap(providerList));
            }

            this._protect = opts.Protect == true;

            // Collapse any 'Alias'es down to URNs. We have to wait until this point to do so
            // because we do not know the default 'name' and 'type' to apply until we are inside the
            // resource constructor.
            var aliases = ImmutableArray.CreateBuilder<Input<Urn>>();
            foreach (var alias in opts.Aliases)
            {
                aliases.Add(CollapseAliasToUrn(alias, name, type, opts.Parent));
            }
            this._aliases = aliases.ToImmutable();

            if (opts.Id != null)
            {
                // If this resource already exists, read its state rather than registering it anew.
                if (!custom)
                {
                    throw new ResourceException(
                        "Cannot read an existing resource unless it has a custom provider", opts.Parent);
                }

                throw new NotSupportedException("Reading is not yet implemented");
                // Deployment.Instance.ReadResource(this, type, name, args, opts);
            }
            else
            {
                // Kick off the resource registration.  If we are actually performing a deployment,
                // this resource's properties will be resolved asynchronously after the operation
                // completes, so that dependent computations resolve normally.  If we are just
                // planning, on the other hand, values will never resolve.
                Deployment.Instance.RegisterResource(this, custom, args, opts);
            }
        }

        protected void OnConstructorCompleted()
            => _onConstructorFinished.TrySetResult(true);

        /// <summary>
        /// Fetches the provider for the given module member, if any.
        /// </summary>
        public ProviderResource? GetProvider(string moduleMember)
        {
            var memComponents = moduleMember.Split(":");
            if (memComponents.Length != 3)
            {
                return null;
            }

            this._providers.TryGetValue(memComponents[0], out var result);
            return result;
        }

        private static Output<Urn> CollapseAliasToUrn(
            Input<UrnOrAlias> alias,
            string defaultName,
            string defaultType,
            Resource? defaultParent)
        {
            return alias.ToOutput().Apply(a =>
            {
                if (a.Urn != null)
                {
                    return Output.Create(a.Urn);
                }

                var alias = a.Alias!;
                var name = alias.Name.HasValue ? alias.Name.Value : defaultName;
                var type = alias.Type.HasValue ? alias.Type.Value : defaultType;
                var project = alias.Project.HasValue ? alias.Project.Value : Deployment.Instance.Options.Project;
                var stack = alias.Stack.HasValue ? alias.Stack.Value : Deployment.Instance.Options.Stack;

                if (alias.Parent.HasValue && alias.ParentUrn.HasValue)
                    throw new ArgumentException("Alias cannot specify Parent and ParentUrn at the same time.");

                Resource? parent;
                Input<Urn>? parentUrn;
                if (alias.Parent.HasValue)
                {
                    parent = alias.Parent.Value;
                    parentUrn = null;
                }
                else if (alias.ParentUrn.HasValue)
                {
                    parent = null;
                    parentUrn = alias.ParentUrn.Value;
                }
                else
                {
                    parent = defaultParent;
                    parentUrn = null;
                }

                if (name == null)
                    throw new Exception("No valid 'Name' passed in for alias.");

                if (type == null)
                    throw new Exception("No valid 'type' passed in for alias.");

                return Pulumi.Urn.Create(name, type, parent, parentUrn, project, stack);
            });
        }

        private static ImmutableDictionary<string, ProviderResource> ConvertToProvidersMap(List<ProviderResource>? providers)
        {
            var result = ImmutableDictionary.CreateBuilder<string, ProviderResource>();
            if (providers != null)
            {
                foreach (var provider in providers)
                {
                    result[provider.Package] = provider;
                }
            }

            return result.ToImmutable();
        }
    }
}
