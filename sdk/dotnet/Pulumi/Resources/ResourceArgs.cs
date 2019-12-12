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

        protected ResourceArgs()
        {
        }

        private protected override void ValidateMember(Type memberType, string fullName)
        {
            if (!typeof(IInput).IsAssignableFrom(memberType))
            {
                throw new InvalidOperationException($"{fullName} must be an Input<T>");
            }
        }

        private class EmptyResourceArgs : ResourceArgs
        {
        }
    }
}
