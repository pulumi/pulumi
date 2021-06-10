// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// Base type for all invoke argument classes.
    /// </summary>
    public abstract class InvokeArgs : InputArgs
    {
        public static readonly InvokeArgs Empty = new EmptyInvokeArgs();

        private protected override void ValidateMember(Type memberType, string fullName)
        {
            if (typeof(IInput).IsAssignableFrom(memberType))
            {
                throw new InvalidOperationException($"{fullName} must not be an Input<T>");
            }
        }

        private class EmptyInvokeArgs : InvokeArgs
        {
        }
    }
}
