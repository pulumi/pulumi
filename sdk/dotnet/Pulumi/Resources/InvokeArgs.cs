// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Base type for all invoke argument classes.
    /// </summary>
    public abstract class InvokeArgs : InputArgs
    {
        public static readonly InvokeArgs Empty = new EmptyInvokeArgs();

        private sealed class EmptyInvokeArgs : InvokeArgs
        {
        }
    }
}
