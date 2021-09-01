// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi
{
    /// <summary>
    /// Base type for all call argument classes.
    /// </summary>
    public abstract class CallArgs : InputArgs
    {
        public static readonly CallArgs Empty = new EmptyCallArgs();

        private protected override void ValidateMember(Type memberType, string fullName)
        {
            // No validation. A member may or may not be IInput.
        }

        private sealed class EmptyCallArgs : CallArgs
        {
        }
    }
}
