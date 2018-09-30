using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi {

    public abstract class CustomResource : Resource {
        #pragma warning disable CS0649
        // This is set by reflection to maintain a type safe external API
        private IO<string> m_id;
        #pragma warning restore CS0649
        public IO<string> Id { get { return m_id; } }
        protected CustomResource(string type, string name, Dictionary<string, IO<Value>> props, ResourceOptions opts = default(ResourceOptions))
            : base(type, name, props, opts) {
        }
    }
}