using System;
using System.Collections.Generic;

namespace Pulumi
{

    public class ComponentResource : Resource
    {
        public ComponentResource(string type, string name, Dictionary<string, object> properties = null, ResourceOptions options = default(ResourceOptions))
        {
            Register(type, name, false, properties, options);
        }
    }
}