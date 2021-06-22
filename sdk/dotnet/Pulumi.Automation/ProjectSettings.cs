// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.IO;

namespace Pulumi.Automation
{
    /// <summary>
    /// A Pulumi project manifest. It describes metadata applying to all sub-stacks created from the project.
    /// </summary>
    public class ProjectSettings
    {
        internal static IEqualityComparer<ProjectSettings> Comparer { get; } = new ProjectSettingsComparer();

        public string Name { get; set; }

        public ProjectRuntime Runtime { get; set; }

        public string? Main { get; set; }

        public string? Description { get; set; }

        public string? Author { get; set; }

        public string? Website { get; set; }

        public string? License { get; set; }

        public string? Config { get; set; }

        public ProjectTemplate? Template { get; set; }

        public ProjectBackend? Backend { get; set; }

        public ProjectSettings(
            string name,
            ProjectRuntime runtime)
        {
            this.Name = name;
            this.Runtime = runtime;
        }

        public ProjectSettings(
            string name,
            ProjectRuntimeName runtime)
            : this(name, new ProjectRuntime(runtime))
        {
        }

        internal static ProjectSettings Default(string name) =>
            new ProjectSettings(name, new ProjectRuntime(ProjectRuntimeName.Dotnet)) { Main = Directory.GetCurrentDirectory() };

        internal bool IsDefault
        {
            get
            {
                return Comparer.Equals(this, Default(this.Name));
            }
        }

        private sealed class ProjectSettingsComparer : IEqualityComparer<ProjectSettings>
        {
            bool IEqualityComparer<ProjectSettings>.Equals(ProjectSettings? x, ProjectSettings? y)
            {
                if (x == null)
                {
                    return y == null;
                }

                if (y == null)
                {
                    return false;
                }

                if (ReferenceEquals(x, y))
                {
                    return true;
                }

                return x.Name == y.Name &&
                        ProjectRuntime.Comparer.Equals(x.Runtime, y.Runtime) &&
                        x.Main == y.Main &&
                        x.Description == y.Description &&
                        x.Author == y.Author &&
                        x.Website == y.Website &&
                        x.License == y.License &&
                        x.Config == y.Config &&
                        ProjectTemplate.Comparer.Equals(x.Template, y.Template) &&
                        ProjectBackend.Comparer.Equals(x.Backend, y.Backend);
            }

            int IEqualityComparer<ProjectSettings>.GetHashCode(ProjectSettings obj)
            {
                // fields with custom Comparer skipped for efficiency
                return HashCode.Combine(
                    obj.Name,
                    obj.Main,
                    obj.Description,
                    obj.Author,
                    obj.Website,
                    obj.License,
                    obj.Config,
                    obj.Backend
                );
            }
        }
    }
}
