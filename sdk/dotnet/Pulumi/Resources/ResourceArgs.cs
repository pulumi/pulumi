// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Base type for all resource argument classes.
    /// </summary>
    public abstract class ResourceArgs : InputArgs
    {
        public static readonly ResourceArgs Empty = new EmptyResourceArgs();

        private sealed class EmptyResourceArgs : ResourceArgs
        {
        }
    }
}
