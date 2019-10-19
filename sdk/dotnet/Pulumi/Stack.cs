// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Stack is the root resource for a Pulumi stack. Before invoking the `init` callback, it
    /// registers itself as the root resource with the Pulumi engine.
    /// 
    /// An instance of this will be automatically created when any <see
    /// cref="Deployment.Run(Action)"/> overload is called.
    /// </summary>
    public class Stack : ComponentResource
    {
        // public static Stack Instance { get; private set; }

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

        /// <summary>
        /// rootPulumiStackTypeName is the type name that should be used to construct the root
        /// component in the tree of Pulumi resources allocated by a deployment.This must be kept up
        /// to date with `github.com/pulumi/pulumi/pkg/resource/stack.RootPulumiStackTypeName`.
        /// </summary>
        internal const string _rootPulumiStackTypeName = "pulumi:pulumi:Stack";

        /// <summary>
        /// The outputs of this stack, if the `init` callback exited normally.
        /// </summary>
        public readonly Output<IDictionary<string, object>> Outputs =
            Output.Create<IDictionary<string, object>>(ImmutableDictionary<string, object>.Empty);

        public Stack(Func<Task<IDictionary<string, object>>> init)
            : base(_rootPulumiStackTypeName, $"{Deployment.Instance.Options.Project}-{Deployment.Instance.Options.Stack}")
        {
            Deployment.Instance.Stack = this;

            try
            {
                this.Outputs = Output.Create(RunInit(init));
            }
            finally
            {
                this.RegisterOutputs(this.Outputs);
            }
        }

        private async Task<IDictionary<string, object>> RunInit(Func<Task<IDictionary<string, object>>> init)
        {
            // Ensure we are known as the root resource.  This is needed before we execute any user
            // code as many codepaths will request the root resource.
            await Deployment.Instance.SetRootResourceAsync(this);

            var dictionary = await init();
            return dictionary == null
                ? ImmutableDictionary<string, object>.Empty
                : dictionary.ToImmutableDictionary();
        }
    }
}
