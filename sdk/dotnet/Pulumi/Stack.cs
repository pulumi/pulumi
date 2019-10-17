// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    public class Stack
    {
        public static string Current => throw new NotImplementedException();

        /// <summary>
        /// Constant to represent the 'root stack' resource for a Pulumi application.  The purpose
        /// of this is solely to make it easy to write an <see cref="Alias"/> like so:
        ///
        /// <c>aliases = { new Alias { Parent = Pulumi.Stack.Root } }</c>
        ///
        /// This indicates that the prior name for a resource was created based on it being parented
        /// directly by the stack itself and no other resources.  Note: this is equivalent to:
        ///
        /// <c>aliases = { new Alias { Parent = null } }</c>
        ///
        /// However, the former form is preferable as it is more self-descriptive, while the latter
        /// may look a bit confusing and may incorrectly look like something that could be removed
        /// without changing semantics.
        /// </summary>
        public static readonly Resource Root = null!;
    }
}
