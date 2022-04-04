// Copyright 2016-2022, Pulumi Corporation

using System.Collections;
using System.Collections.Generic;

namespace Pulumi
{
    /// <summary>
    /// A type-safe wrapper around <see cref="IDictionary{String, Object}"/> for generating stack outputs from created resources. 
    /// </summary>
    public class Outputs : IEnumerable<KeyValuePair<string, object?>>
    {
        // Internally, we keep track of a dictionary which keeps track of untyped values (objects)
        private readonly Dictionary<string, object?> outputs = new Dictionary<string, object?>();

        public void Add(string outputName, string value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Add(string outputName, int value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Add(string outputName, bool value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Add(string outputName, double value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public void Add(string outputName, Output<string> value)
        {
            outputs[outputName] = value;
        }

        public void Add(string outputName, Output<int> value)
        {
            outputs[outputName] = value;
        }

        public void Add(string outputName, Output<bool> value)
        {
            outputs[outputName] = value;
        }

        public void Add(string outputName, Output<double> value)
        {
            outputs[outputName] = Output.Create(value);
        }

        public IDictionary<string, object?> AsDictionary()
        {
            return outputs;
        }

        public IEnumerator<KeyValuePair<string, object?>> GetEnumerator() => outputs.GetEnumerator();

        IEnumerator IEnumerable.GetEnumerator() => outputs.GetEnumerator();
    }
}