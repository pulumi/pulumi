// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using System.Threading.Tasks;
using System.Transactions;

namespace Pulumi
{
    /// <summary>
    /// Stack is the root resource for a Pulumi stack. Before invoking the `init` callback, it
    /// registers itself as the root resource with the Pulumi engine.
    /// </summary>
    public class Stack : ComponentResource
    {
        public static Stack Instance { get; private set; }

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

        public readonly string Project;
        public readonly string Name;

        /// <summary>
        /// The outputs of this stack, if the `init` callback exited normally.
        /// </summary>
        public readonly Output<ImmutableDictionary<string, object>> Outputs =
            Output.Create(ImmutableDictionary<string, object>.Empty);

        public Stack(Func<Task<InputMap<string, object>>> init)
            : base(_rootPulumiStackTypeName, $"{GlobalOptions.Instance.Project}-{GlobalOptions.Instance.Stack}")
        {
            this.Project = GlobalOptions.Instance.Project;
            this.Name = GlobalOptions.Instance.Stack;

            if (Instance != null)
            {
                throw new InvalidOperationException("Cannot make multiple instances of the Stack resource");
            }

            Instance = this;

            try
            {
                this.Outputs = Output.Create(init()).Apply(m => m.GetInnerMap());
            }
            finally
            {
                this.RegisterOutputs(this.Outputs);
            }
        }
    }
}

//    ///**
//    // * The outputs of this stack, if the `init` callback exited normally.
//    // */
//    //public readonly outputs: Output<Inputs | undefined>;

//    //constructor(init: () => Inputs)
//    //{
//    //    super(rootPulumiStackTypeName, `${ getProject()}
//    //    -${ getStack()}`);
//    //    this.outputs = output(this.runInit(init));
//    //}

//    /**
//     * runInit invokes the given init callback with this resource set as the root resource. The return value of init is
//     * used as the stack's output properties.
//     *
//     * @param init The callback to run in the context of this Pulumi stack
//     */
//    private async runInit(init: () => Inputs): Promise<Inputs | undefined> {
//        const parent = await getRootResource();
//        if (parent) {
//            throw new Error("Only one root Pulumi Stack may be active at once");
//}
//await setRootResource(this);

//// Set the global reference to the stack resource before invoking this init() function
//stackResource = this;

//        let outputs: Inputs | undefined;
//        try {

//    outputs = await massage(init(), []);
//        } finally {
//            // We want to expose stack outputs as simple pojo objects (including Resources).  This
//            // helps ensure that outputs can point to resources, and that that is stored and
//            // presented as something reasonable, and not as just an id/urn in the case of
//            // Resources.
//            super.registerOutputs(outputs);
//        }

//        return outputs;
//    }
//}
