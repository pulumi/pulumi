// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// A description of the Project's program runtime and associated metadata.
    /// </summary>
    public class ProjectRuntime
    {
        internal static IEqualityComparer<ProjectRuntime> Comparer { get; } = new ProjectRuntimeComparer();

        public ProjectRuntimeName Name { get; set; }

        public ProjectRuntimeOptions? Options { get; set; }

        public ProjectRuntime(ProjectRuntimeName name)
        {
            this.Name = name;
        }

        private sealed class ProjectRuntimeComparer : IEqualityComparer<ProjectRuntime>
        {
            bool IEqualityComparer<ProjectRuntime>.Equals(ProjectRuntime? x, ProjectRuntime? y)
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

                return x.Name == y.Name && ProjectRuntimeOptions.Comparer.Equals(x.Options, y.Options);
            }

            int IEqualityComparer<ProjectRuntime>.GetHashCode(ProjectRuntime obj)
            {
                return HashCode.Combine(
                    obj.Name,
                    obj.Options != null ? ProjectRuntimeOptions.Comparer.GetHashCode(obj.Options) : 0
                );
            }
        }
    }
}
