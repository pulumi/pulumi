// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// Manages a reference to a Pulumi stack and provides access to the referenced stack's outputs.
    /// </summary>
    public class StackReference : CustomResource
    {
        /// <summary>
        /// The name of the referenced stack.
        /// </summary>
        [Output("name")]
        public Output<string> Name { get; private set; } = null!;

        /// <summary>
        /// The outputs of the referenced stack.
        /// </summary>
        [Output("outputs")]
        public Output<ImmutableDictionary<string, object>> Outputs { get; private set; } = null!;

        /// <summary>
        /// The names of any stack outputs which contain secrets.
        /// </summary>
        [Output("secretOutputNames")]
        public Output<ImmutableArray<string>> SecretOutputNames { get; private set; } = null!;

        /// <summary>
        /// Create a <see cref="StackReference"/> resource with the given unique name, arguments, and options.
        /// <para />
        /// If args is not specified, the name of the referenced stack will be the name of the StackReference resource.
        /// </summary>
        /// <param name="name">The unique name of the stack reference.</param>
        /// <param name="args">The arguments to use to populate this resource's properties.</param>
        /// <param name="options">A bag of options that control this resource's behavior.</param>
        public StackReference(string name, StackReferenceArgs? args = null, CustomResourceOptions? options = null)
            : base("pulumi:pulumi:StackReference",
                  name,
                  new StackReferenceArgs { Name = args?.Name ?? name },
                  CustomResourceOptions.Merge(options, new CustomResourceOptions { Id = args?.Name ?? name }))
        {
        }

        /// <summary>
        /// Fetches the value of the named stack output, or null if the stack output was not found.
        /// </summary>
        /// <param name="name">The name of the stack output to fetch.</param>
        /// <returns>An <see cref="Output{T}"/> containing the requested value.</returns>
        public Output<object?> GetOutput(Input<string> name)
        {
            // Note that this is subltly different from "Apply" here. A default "Apply" will set the secret bit if any
            // of the inputs are a secret, and this.Outputs is always a secret if it contains any secrets. We do this dance
            // so we can ensure that the Output we return is not needlessly tainted as a secret.
            var inputs = (Input<ImmutableDictionary<string, object>>)this.Outputs;
            var value = Output.Tuple(name, inputs).Apply(v =>
                v.Item2.TryGetValue(v.Item1, out var result) ? result : null);

            return value.WithIsSecret(IsSecretOutputName(name));
        }

        /// <summary>
        /// Fetches the value of the named stack output, or throws an error if the output was not found.
        /// </summary>
        /// <param name="name">The name of the stack output to fetch.</param>
        /// <returns>An <see cref="Output{T}"/> containing the requested value.</returns>
        public Output<object> RequireOutput(Input<string> name)
        {
            var inputs = (Input<ImmutableDictionary<string, object>>)this.Outputs;
            var stackName = (Input<string>)this.Name;
            var value = Output.Tuple(name, stackName, inputs).Apply(v =>
                v.Item3.TryGetValue(v.Item1, out var result)
                    ? result
                    : throw new KeyNotFoundException(
                        $"Required output '{v.Item1}' does not exist on stack '{v.Item2}'."));

            return value.WithIsSecret(IsSecretOutputName(name));
        }

        /// <summary>
        /// Fetches the value of the named stack output. May return null if the value is
        /// not known for some reason.
        /// <para />
        /// This operation is not supported (and will throw) for secret outputs.
        /// </summary>
        /// <param name="name">The name of the stack output to fetch.</param>
        /// <returns>The value of the referenced stack output.</returns>
        public async Task<object?> GetValueAsync(Input<string> name)
        {
            var output = this.GetOutput(name);
            var data = await output.DataTask.ConfigureAwait(false);
            if (data.IsSecret)
            {
                throw new InvalidOperationException(
                    "Cannot call 'GetOutputValueAsync' if the referenced stack has secret outputs. Use 'GetOutput' instead.");
            }

            return data.Value;
        }

        /// <summary>
        /// Fetches the value promptly of the named stack output. Throws an error if the stack output is
        /// not found.
        /// <para />
        /// This operation is not supported (and will throw) for secret outputs.
        /// </summary>
        /// <param name="name">The name of the stack output to fetch.</param>
        /// <returns>The value of the referenced stack output.</returns>
        public async Task<object> RequireValueAsync(Input<string> name)
        {
            var output = this.RequireOutput(name);
            var data = await output.DataTask.ConfigureAwait(false);
            if (data.IsSecret)
            {
                throw new InvalidOperationException(
                    "Cannot call 'RequireOutputValueAsync' if the referenced stack has secret outputs. Use 'RequireOutput' instead.");
            }

            return data.Value;
        }

        private async Task<bool> IsSecretOutputName(Input<string> name)
        {
            var nameOutput = await name.ToOutput().DataTask.ConfigureAwait(false);
            var secretOutputNamesData = await this.SecretOutputNames.DataTask.ConfigureAwait(false);

            // If either the name or set of secret outputs is unknown, we can't do anything smart, so we just copy the
            // secretness from the entire outputs value.
            if (!(nameOutput.IsKnown && secretOutputNamesData.IsKnown))
            {
                return (await this.Outputs.DataTask.ConfigureAwait(false)).IsSecret;
            }

            // Otherwise, if we have a list of outputs we know are secret, we can use that list to determine if this
            // output should be secret.
            var names = secretOutputNamesData.Value;
            return names.Contains(nameOutput.Value);
        }
    }

    /// <summary>
    /// The set of arguments for constructing a StackReference resource.
    /// </summary>
    public class StackReferenceArgs : ResourceArgs
    {
        /// <summary>
        /// The name of the stack to reference.
        /// </summary>
        [Input("name", required: true)]
        public Input<string>? Name { get; set; }
    }
}
