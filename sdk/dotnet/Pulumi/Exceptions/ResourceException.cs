// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// ResourceException can be used for terminating a program abruptly, specifically associating the
    /// problem with a Resource.Depending on the nature of the problem, clients can choose whether
    /// or not a call stack should be returned as well. This should be very rare, and would only
    /// indicate no usefulness of presenting that stack to the user.
    /// </summary>
    public class ResourceException : Exception
    {
        public readonly Resource? Resource;
        public readonly bool HideStack;

        public ResourceException(string message, Resource? resource, bool hideStack = false) : base(message)
        {
            this.Resource = resource;
            this.HideStack = hideStack;
        }
    }
}
