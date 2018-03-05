using System;
using System.Collections.Generic;

namespace Pulumi {

    public class CustomResource : Resource {
        public string Id {get; internal set;}

        public CustomResource(string type, string name, Dictionary<string, object> properties = null, ResourceOptions options = default(ResourceOptions))  {
            var res = Register(type, name, true, properties, options);

            Id = res.Id;
        }
    }
}