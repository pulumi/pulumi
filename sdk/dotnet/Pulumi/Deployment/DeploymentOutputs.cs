using System;
using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// A type-safe wrapper around <see cref="IDictionary{String, Object}"/> for generating stack outputs from created resources. 
    /// </summary>
    public class DeploymentOutputs
    {
        private readonly Dictionary<string, object?> outputs;

        private DeploymentOutputs()
        {
            outputs = new Dictionary<string, object?>();
        }

        public static DeploymentOutputs Create()
        {
            return new DeploymentOutputs();
        }

        public void Export<T>(string outputName, Output<T> value)
        {
            outputs[outputName] = value;
        }

        public void Export(string outputName, string value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Export(string outputName, int value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Export(string outputName, bool value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public IDictionary<string, object?> AsDictionary()
        {
            return outputs;
        }
    }
}