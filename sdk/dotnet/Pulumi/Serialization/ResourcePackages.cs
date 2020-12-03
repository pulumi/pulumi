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
    public interface IResourcePackage
    {
        string Name { get; }
        string? Version { get; }
        ProviderResource ConstructProvider(string name, string type, string urn);
        Resource Construct(string name, string type, string urn);
    }

    internal static class ResourcePackages
    {
        private static ImmutableList<IResourcePackage>? _resourcePackages;
        private static readonly object _resourcePackagesLock = new object();

        internal static bool TryGetResourcePackage(string name, string? version, [NotNullWhen(true)] out IResourcePackage? package)
        {
            lock (_resourcePackagesLock)
            {
                _resourcePackages ??= DiscoverResourcePackages();
            }

            var minimalVersion = version != null ? SemVersion.Parse(version) : new SemVersion(0);
            var matches =
                from p in _resourcePackages
                where p.Name == name
                let packageVersion = p.Version != null ? SemVersion.Parse(p.Version) : minimalVersion
                where packageVersion >= minimalVersion
                where (version == null || p.Version == null || minimalVersion.Major == packageVersion.Major)
                orderby packageVersion descending
                select p;
            
            package = matches.FirstOrDefault();
            return package != null;
        }

        private static ImmutableList<IResourcePackage> DiscoverResourcePackages()
        {
            return LoadReferencedAssemblies()
                .SelectMany(s => s.GetTypes())
                .Where(typeof(IResourcePackage).IsAssignableFrom)
                .SelectMany(type => type.GetConstructors().Where(c => c.GetParameters().Length == 0))
                .Select(c => c.Invoke(new object[0]))
                .Cast<IResourcePackage>()
                .ToImmutableList();
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
