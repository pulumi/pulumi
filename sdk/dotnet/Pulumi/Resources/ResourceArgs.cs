// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// Base type for all resource argument classes.
    /// </summary>
    public abstract class ResourceArgs : InputArgs
    {
        public static readonly ResourceArgs Empty = new EmptyResourceArgs();

        private protected override void ValidateMember(Type memberType, string fullName)
        {
            // No validation. A member may or may not be IInput.
        }

        private class EmptyResourceArgs : ResourceArgs
        {
        }
    }
}
