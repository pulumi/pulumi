// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// A placeholder config value for a project template.
    /// </summary>
    public class ProjectTemplateConfigValue
    {
        internal static IEqualityComparer<ProjectTemplateConfigValue> Comparer { get; } = new ProjectTemplateConfigValueComparer();

        public string? Description { get; set; }

        public string? Default { get; set; }

        public bool? Secret { get; set; }

        private sealed class ProjectTemplateConfigValueComparer : IEqualityComparer<ProjectTemplateConfigValue>
        {
            bool IEqualityComparer<ProjectTemplateConfigValue>.Equals(ProjectTemplateConfigValue? x, ProjectTemplateConfigValue? y)
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

                return x.Description == y.Description && x.Default == y.Default && x.Secret == y.Secret;
            }

            int IEqualityComparer<ProjectTemplateConfigValue>.GetHashCode(ProjectTemplateConfigValue obj)
            {
                return HashCode.Combine(obj.Description, obj.Default, obj.Secret);
            }
        }
    }
}
