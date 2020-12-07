// Copyright 2016-2020, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Reflection;
using Semver;

namespace Pulumi
{
    internal static class ResourcePackages
    {
        private static ImmutableDictionary<string, ImmutableList<(string?, Type)>>? _resourceTypes;
        private static readonly object _resourceTypesLock = new object();
        
        internal static Resource Construct(string type, string version, string urn)
        {
            if (!TryGetResourceType(type, version, out var resourceType))
            {
                throw new InvalidOperationException($"Unable to deserialize resource {urn}.");
            }

            var urnParts = urn.Split("::");
            var urnName = urnParts[3];
            var constructorInfo = resourceType.GetConstructors().Single(c => c.GetParameters().Length == 3);
            return (Resource)constructorInfo.Invoke(new[] {urnName, (object?)null, new CustomResourceOptions {Urn = urn}});
        }

        internal static bool TryGetResourceType(string name, string? version, [NotNullWhen(true)] out Type? type)
        {
            lock (_resourceTypesLock)
            {
                _resourceTypes ??= DiscoverResourceTypes();
            }

            var minimalVersion = version != null ? SemVersion.Parse(version) : new SemVersion(0);
            var yes = _resourceTypes.TryGetValue(name, out var types);
            if (!yes)
            {
                type = null;
                return false;
            }
            
            var matches =
                    from vt in types
                    let resourceVersion = vt.Item1 != null ? SemVersion.Parse(vt.Item1) : minimalVersion
                    where resourceVersion >= minimalVersion
                    where (version == null || vt.Item1 == null || minimalVersion.Major == resourceVersion.Major)
                    orderby resourceVersion descending
                    select vt.Item2;
            
            type = matches.FirstOrDefault();
            return type != null;
        }
        
        private static ImmutableDictionary<string, ImmutableList<(string?, Type)>> DiscoverResourceTypes()
        {
            var pairs =
                from a in LoadReferencedAssemblies()
                from t in a.GetTypes()
                where typeof(CustomResource).IsAssignableFrom(t)
                let attr = t.GetCustomAttribute<ResourceTypeAttribute>()
                where attr != null
                let ut = a.GetType($"{a.GetName().Name}.Utilities")
                let versionProp = ut?.GetProperty("Version", BindingFlags.Static | BindingFlags.Public | BindingFlags.GetProperty)
                let assemblyVersion = (string?)versionProp?.GetValue(null)
                let version = attr.Version ?? assemblyVersion
                let versionType = (version, t)
                group versionType by attr.Type into g
                select new { g.Key, Items = g};
            return pairs.ToImmutableDictionary(v => v.Key, v => v.Items.ToImmutableList());
        }
        
        // Assemblies are loaded on demand, so it could be that some assemblies aren't yet loaded to the current
        // app domain at the time of discovery. This method iterates through the list of referenced assemblies
        // recursively.
        // Note: If an assembly is referenced but its types are never used anywhere in the program, that reference
        // will be optimized out and won't appear in the result of the enumeration.
        private static IEnumerable<Assembly> LoadReferencedAssemblies()
        {
            var yieldedAssemblies = new HashSet<string>();
            var assembliesToCheck = new Queue<Assembly>();

            foreach (var assembly in AppDomain.CurrentDomain.GetAssemblies())
            {
                if (PossibleMatch(assembly.GetName()))
                {
                    assembliesToCheck.Enqueue(assembly!);
                }
            }

            while (assembliesToCheck.Any())
            {
                Assembly assemblyToCheck = assembliesToCheck.Dequeue();
                if (yieldedAssemblies.Contains(assemblyToCheck.FullName!))
                    continue;

                yieldedAssemblies.Add(assemblyToCheck.FullName!);
                yield return assemblyToCheck;
                
                foreach (var reference in assemblyToCheck.GetReferencedAssemblies())
                {
                    if (PossibleMatch(reference))
                    {
                        var assembly = Assembly.Load(reference);
                        assembliesToCheck.Enqueue(assembly);
                    }
                }
            }

            static bool PossibleMatch(AssemblyName? assembly) => assembly != null && !assembly.FullName.StartsWith("System", StringComparison.Ordinal);
        }
    }
}
