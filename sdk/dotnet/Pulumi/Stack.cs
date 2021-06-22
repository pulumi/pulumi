// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Stack is the root resource for a Pulumi stack. Derive from this class to create your
    /// stack definitions.
    /// </summary>
    public class Stack : ComponentResource
    {
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
        internal static Resource? Root { get; } = null;

        /// <summary>
        /// <see cref="_rootPulumiStackTypeName"/> is the type name that should be used to construct
        /// the root component in the tree of Pulumi resources allocated by a deployment.This must
        /// be kept up to date with
        /// <c>github.com/pulumi/pulumi/sdk/v3/go/common/resource/stack.RootStackType</c>.
        /// </summary>
        internal const string _rootPulumiStackTypeName = "pulumi:pulumi:Stack";

        /// <summary>
        /// The outputs of this stack, if the <c>init</c> callback exited normally.
        /// </summary>
        internal Output<IDictionary<string, object?>> Outputs { get; private set; } =
            Output.Create<IDictionary<string, object?>>(ImmutableDictionary<string, object?>.Empty);

        /// <summary>
        /// Create a Stack with stack resources defined in derived class constructor.
        /// </summary>
        public Stack(StackOptions? options = null)
            : base(_rootPulumiStackTypeName, 
                $"{Deployment.Instance.ProjectName}-{Deployment.Instance.StackName}",
                ConvertOptions(options))
        {
            Deployment.InternalInstance.Stack = this;
        }

        /// <summary>
        /// Create a Stack with stack resources created by the <c>init</c> callback.
        /// An instance of this will be automatically created when any <see
        /// cref="Deployment.RunAsync(Action)"/> overload is called.
        /// </summary>
        internal Stack(Func<Task<IDictionary<string, object?>>> init, StackOptions? options) : this(options)
        {
            try
            {
                this.Outputs = Output.Create(RunInitAsync(init));
            }
            finally
            {
                this.RegisterOutputs(this.Outputs);
            }
        }

        /// <summary>
        /// Inspect all public properties of the stack to find outputs. Validate the values and register them as stack outputs.
        /// </summary>
        internal void RegisterPropertyOutputs()
        {
            var outputs = (from property in this.GetType().GetProperties(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly)
                           let attr = property.GetCustomAttribute<OutputAttribute>()
                           where attr != null
                           let name = attr?.Name ?? property.Name
                           select new KeyValuePair<string, object?>(name, property.GetValue(this))).ToList();

            // Check that none of the values are null: catch unassigned outputs
            var nulls = (from kv in outputs
                         where kv.Value == null
                         select kv.Key).ToList();
            if (nulls.Any())
            {
                var message = $"Output(s) '{string.Join(", ", nulls)}' have no value assigned. [Output] attributed properties must be assigned inside Stack constructor.";
                throw new RunException(message);
            }

            // Check that all the values are Output<T>
            var wrongTypes = (from kv in outputs
                              let type = kv.Value.GetType()
                              let isOutput = type.IsGenericType && type.GetGenericTypeDefinition() == typeof(Output<>)
                              where !isOutput
                              select kv.Key).ToList();
            if (wrongTypes.Any())
            {
                var message = $"Output(s) '{string.Join(", ", wrongTypes)}' have incorrect type. [Output] attributed properties must be instances of Output<T>.";
                throw new RunException(message);
            }

            IDictionary<string, object?> dict = new Dictionary<string, object?>(outputs);
            this.Outputs = Output.Create(dict);
            this.RegisterOutputs(this.Outputs);
        }

        private static async Task<IDictionary<string, object?>> RunInitAsync(Func<Task<IDictionary<string, object?>>> init)
        {
            var dictionary = await init().ConfigureAwait(false);
            return dictionary.ToImmutableDictionary();
        }
        
        private static ComponentResourceOptions? ConvertOptions(StackOptions? options)
        {
            if (options == null)
                return null;
            
            return new ComponentResourceOptions
            {
                ResourceTransformations = options.ResourceTransformations
            };
        }
    }
}
