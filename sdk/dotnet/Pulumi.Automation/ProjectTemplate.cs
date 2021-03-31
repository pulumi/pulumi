// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// A template used to seed new stacks created from this project.
    /// </summary>
    public class ProjectTemplate
    {
        public string? Description { get; set; }

        public string? QuickStart { get; set; }

        public IDictionary<string, ProjectTemplateConfigValue>? Config { get; set; }

        public bool? Important { get; set; }

        internal static IEqualityComparer<ProjectTemplate> Comparer { get; } = new ProjectTemplateComparer();

        private sealed class ProjectTemplateComparer : IEqualityComparer<ProjectTemplate>
        {

            private IEqualityComparer<IDictionary<string, ProjectTemplateConfigValue>> _configComparer =
                new DictionaryContentsComparer<string, ProjectTemplateConfigValue>(
                    EqualityComparer<string>.Default,
                    ProjectTemplateConfigValue.Comparer);

            bool IEqualityComparer<ProjectTemplate>.Equals(ProjectTemplate? x, ProjectTemplate? y)
            {
                if (x == null)
                {
                    return y == null;
                }

                if (y == null)
                {
                    return x == null;
                }

                return x.Description == y.Description
                    && x.QuickStart == y.QuickStart
                    && x.Important == y.Important
                    && _configComparer.Equals(x.Config, y.Config);
            }

            int IEqualityComparer<ProjectTemplate>.GetHashCode(ProjectTemplate obj)
            {
                var hash = new HashCode();
                hash.Add(obj.Description);
                hash.Add(obj.QuickStart);
                hash.Add(obj.Important);
                // omit hashing Config dict for efficiency
                return hash.ToHashCode();
            }
        }
    }
}
