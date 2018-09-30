using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{

    public class ComponentResource : Resource
    {
        public ComponentResource(string type, string name, ResourceOptions opts = default(ResourceOptions))
            : base(type, name, null, opts) {
        }
        protected void RegisterOutputs(Dictionary<string, IO<Value>> outputs) {
            var ignore = Runtime.RegisterResourceOutputsAsync(this, outputs);
        }
    }
}