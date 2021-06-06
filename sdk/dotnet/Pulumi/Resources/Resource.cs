// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Resource represents a class whose CRUD operations are implemented by a provider plugin.
    /// </summary>
    public class Resource
    {
        private readonly string _type;
        private readonly string _name;

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
        internal HashSet<Resource> ChildResources { get; } = new HashSet<Resource>();

        /// <summary>
        /// Urn is the stable logical URN used to distinctly address a resource, both before and
        /// after deployments.
        /// </summary>
        // Set using reflection, so we silence the NRT warnings with `null!`.
        [Output(Constants.UrnPropertyName)]
        public Output<string> Urn { get; private protected set; } = null!;

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
        internal readonly ImmutableArray<Input<string>> _aliases;

        /// <summary>
        /// The type assigned to the resource at construction.
        /// </summary>
        // This is a method and not a property to not collide with potential subclass property names.
        public string GetResourceType() => _type;

        /// <summary>
        /// The name assigned to the resource at construction.
        /// </summary>
        // This is a method and not a property to not collide with potential subclass property names.
        public string GetResourceName() => _name;

        /// <summary>
        /// The set of providers to use for child resources. Keyed by package name (e.g. "aws").
        /// </summary>
        private readonly ImmutableDictionary<string, ProviderResource> _providers;

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
        /// <param name="options">A bag of options that control this resource's behavior.</param>
        /// <param name="remote">True if this is a remote component resource.</param>
        /// <param name="dependency">True if this is a synthetic resource used internally for dependency tracking.</param>
        private protected Resource(
            string type, string name, bool custom,
            ResourceArgs args, ResourceOptions options,
            bool remote = false, bool dependency = false)
        {
            if (dependency)
            {
                _type = "";
                _name = "";
                _protect = false;
                _providers = ImmutableDictionary<string, ProviderResource>.Empty;
                return;
            }

            if (string.IsNullOrEmpty(type))
                throw new ArgumentException("'type' cannot be null or empty.", nameof(type));

            if (string.IsNullOrEmpty(name))
                throw new ArgumentException("'name' cannot be null or empty.", nameof(name));

            // Before anything else - if there are transformations registered, invoke them in order
            // to transform the properties and options assigned to this resource.
            var parent = type == Stack._rootPulumiStackTypeName
                ? null
                : (options.Parent ?? Deployment.InternalInstance.Stack);

            _type = type;
            _name = name;

            var transformations = ImmutableArray.CreateBuilder<ResourceTransformation>();
            transformations.AddRange(options.ResourceTransformations);
            if (parent != null)
            {
                transformations.AddRange(parent._transformations);
            }
            this._transformations = transformations.ToImmutable();

            foreach (var transformation in this._transformations)
            {
                var tres = transformation(new ResourceTransformationArgs(this, args, options));
                if (tres != null)
                {
                    if (tres.Value.Options.Parent != options.Parent)
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

                    args = tres.Value.Args;
                    options = tres.Value.Options;
                }
            }

            // Make a shallow clone of options to ensure we don't modify the value passed in.
            options = options.Clone();
            var componentOpts = options as ComponentResourceOptions;
            var customOpts = options as CustomResourceOptions;

            if (options.Provider != null &&
                componentOpts?.Providers.Count > 0)
            {
                throw new ResourceException("Do not supply both 'provider' and 'providers' options to a ComponentResource.", options.Parent);
            }

            // Check the parent type if one exists and fill in any default options.
            this._providers = ImmutableDictionary<string, ProviderResource>.Empty;

            if (options.Parent != null)
            {
                var parentResource = options.Parent;
                lock (parentResource.ChildResources)
                {
                    parentResource.ChildResources.Add(this);
                }

                options.Protect ??= options.Parent._protect;

                // Make a copy of the aliases array, and add to it any implicit aliases inherited from its parent
                options.Aliases = options.Aliases.ToList();
                foreach (var parentAlias in options.Parent._aliases)
                {
                    options.Aliases.Add(Pulumi.Urn.InheritedChildAlias(name, options.Parent.GetResourceName(), parentAlias, type));
                }

                this._providers = options.Parent._providers;
            }

            if (custom)
            {
                var provider = customOpts?.Provider;
                if (provider == null)
                {
                    if (options.Parent != null)
                    {
                        // If no provider was given, but we have a parent, then inherit the
                        // provider from our parent.

                        options.Provider = options.Parent.GetProvider(type);
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
                        this._providers = this._providers.SetItem(pkg, provider);
                    }
                }
            }
            else
            {
                // Note: we checked above that at most one of options.provider or options.providers
                // is set.

                // If options.provider is set, treat that as if we were given a array of provider
                // with that single value in it.  Otherwise, take the array of providers, convert it
                // to a map and combine with any providers we've already set from our parent.
                var providerList = options.Provider != null
                    ? new List<ProviderResource> { options.Provider }
                    : componentOpts?.Providers;

                this._providers = this._providers.AddRange(ConvertToProvidersMap(providerList));
            }

            this._protect = options.Protect == true;

            // Collapse any 'Alias'es down to URNs. We have to wait until this point to do so
            // because we do not know the default 'name' and 'type' to apply until we are inside the
            // resource constructor.
            var aliases = ImmutableArray.CreateBuilder<Input<string>>();
            foreach (var alias in options.Aliases)
            {
                aliases.Add(CollapseAliasToUrn(alias, name, type, options.Parent));
            }
            this._aliases = aliases.ToImmutable();

            Deployment.InternalInstance.ReadOrRegisterResource(this, remote, urn => new DependencyResource(urn), args, options);
        }

        /// <summary>
        /// Fetches the provider for the given module member, if any.
        /// </summary>
        internal ProviderResource? GetProvider(string moduleMember)
        {
            var memComponents = moduleMember.Split(":");
            if (memComponents.Length != 3)
            {
                return null;
            }

            this._providers.TryGetValue(memComponents[0], out var result);
            return result;
        }

        private static Output<string> CollapseAliasToUrn(
            Input<Alias> alias,
            string defaultName,
            string defaultType,
            Resource? defaultParent)
        {
            return alias.ToOutput().Apply(a =>
            {
                if (a.Urn != null)
                {
                    CheckNull(a.Name, nameof(a.Name));
                    CheckNull(a.Type, nameof(a.Type));
                    CheckNull(a.Project, nameof(a.Project));
                    CheckNull(a.Stack, nameof(a.Stack));
                    CheckNull(a.Parent, nameof(a.Parent));
                    CheckNull(a.ParentUrn, nameof(a.ParentUrn));
                    if (a.NoParent)
                        ThrowAliasPropertyConflict(nameof(a.NoParent));

                    return Output.Create(a.Urn);
                }

                var name = a.Name ?? defaultName;
                var type = a.Type ?? defaultType;
                var project = a.Project ?? Deployment.Instance.ProjectName;
                var stack = a.Stack ?? Deployment.Instance.StackName;

                var parentCount =
                    (a.Parent != null ? 1 : 0) +
                    (a.ParentUrn != null ? 1 : 0) +
                    (a.NoParent ? 1 : 0);

                if (parentCount >= 2)
                {
                    throw new ArgumentException(
$"Only specify one of '{nameof(Alias.Parent)}', '{nameof(Alias.ParentUrn)}' or '{nameof(Alias.NoParent)}' in an {nameof(Alias)}");
                }

                var (parent, parentUrn) = GetParentInfo(defaultParent, a);

                if (name == null)
                    throw new Exception("No valid 'Name' passed in for alias.");

                if (type == null)
                    throw new Exception("No valid 'type' passed in for alias.");

                return Pulumi.Urn.Create(name, type, parent, parentUrn, project, stack);
            });
        }

        private static void CheckNull<T>(T? value, string name) where T : class
        {
            if (value != null)
            {
                ThrowAliasPropertyConflict(name);
            }
        }

        private static void ThrowAliasPropertyConflict(string name)
            => throw new ArgumentException($"{nameof(Alias)} should not specify both {nameof(Alias.Urn)} and {name}");

        private static (Resource? parent, Input<string>? urn) GetParentInfo(Resource? defaultParent, Alias alias)
        {
            if (alias.Parent != null)
                return (alias.Parent, null);

            if (alias.ParentUrn != null)
                return (null, alias.ParentUrn);

            if (alias.NoParent)
                return (null, null);

            return (defaultParent, null);
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
