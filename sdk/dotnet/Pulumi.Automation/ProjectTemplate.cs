// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using Pulumi.Automation.Collections;

namespace Pulumi.Automation
{
    /// <summary>
    /// A template used to seed new stacks created from this project.
    /// </summary>
    public class ProjectTemplate
    {
        internal static IEqualityComparer<ProjectTemplate> Comparer { get; } = new ProjectTemplateComparer();

        public string? Description { get; set; }

        public string? QuickStart { get; set; }

        public IDictionary<string, ProjectTemplateConfigValue>? Config { get; set; }

        public bool? Important { get; set; }

        private sealed class ProjectTemplateComparer : IEqualityComparer<ProjectTemplate>
        {

            private readonly IEqualityComparer<IDictionary<string, ProjectTemplateConfigValue>> _configComparer =
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
                    return false;
                }

                if (ReferenceEquals(x, y))
                {
                    return true;
                }

                return x.Description == y.Description
                    && x.QuickStart == y.QuickStart
                    && x.Important == y.Important
                    && _configComparer.Equals(x.Config, y.Config);
            }

            int IEqualityComparer<ProjectTemplate>.GetHashCode(ProjectTemplate obj)
            {
                // omit hashing Config dict for efficiency
                return HashCode.Combine(obj.Description, obj.QuickStart, obj.Important);
            }
        }
    }
}
