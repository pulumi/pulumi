// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// Configuration for the project's Pulumi state storage backend.
    /// </summary>
    public class ProjectBackend
    {
        internal static IEqualityComparer<ProjectBackend> Comparer { get; } = new ProjectBackendComparer();

        public string? Url { get; set; }

        private sealed class ProjectBackendComparer : IEqualityComparer<ProjectBackend>
        {
            bool IEqualityComparer<ProjectBackend>.Equals(ProjectBackend? x, ProjectBackend? y)
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

                return x.Url == y.Url;
            }

            int IEqualityComparer<ProjectBackend>.GetHashCode(ProjectBackend obj)
            {
                return HashCode.Combine(obj.Url);
            }
        }
    }
}
