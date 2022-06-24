// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// Various configuration options that apply to different language runtimes.
    /// </summary>
    public class ProjectRuntimeOptions
    {
        internal static IEqualityComparer<ProjectRuntimeOptions> Comparer { get; } = new ProjectRuntimeOptionsComparer();

        /// <summary>
        /// Applies to NodeJS projects only.
        /// <para/>
        /// A boolean that controls whether to use ts-node to execute sources.
        /// </summary>
        public bool? TypeScript { get; set; }

        /// <summary>
        /// Applies to Go and .NET project only.
        /// <para/>
        /// Go: A string that specifies the name of a pre-build executable to look for on your path.
        /// <para/>
        /// .NET: A string that specifies the path of a pre-build .NET assembly.
        /// </summary>
        public string? Binary { get; set; }

        /// <summary>
        /// Applies to Python projects only.
        /// <para/>
        /// A string that specifies the path to a virtual environment to use when running the program.
        /// </summary>
        public string? VirtualEnv { get; set; }

        private sealed class ProjectRuntimeOptionsComparer : IEqualityComparer<ProjectRuntimeOptions>
        {
            bool IEqualityComparer<ProjectRuntimeOptions>.Equals(ProjectRuntimeOptions? x, ProjectRuntimeOptions? y)
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

                return x.TypeScript == y.TypeScript && x.Binary == y.Binary && x.VirtualEnv == y.VirtualEnv;
            }

            int IEqualityComparer<ProjectRuntimeOptions>.GetHashCode(ProjectRuntimeOptions obj)
            {
                return HashCode.Combine(obj.TypeScript, obj.Binary, obj.VirtualEnv);
            }
        }
    }
}
