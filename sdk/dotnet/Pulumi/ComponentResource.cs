// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading.Tasks;

namespace Pulumi
{
    /// <summary>
    /// A <see cref="Resource"/> that aggregates one or more other child resources into a higher
    /// level abstraction.The component resource itself is a resource, but does not require custom
    /// CRUD operations for provisioning.
    /// </summary>
    public class ComponentResource : Resource
    {
    }
}

//using System;
//using System.Collections.Generic;

//namespace Pulumi
//{

//    public class ComponentResource : Resource
//    {
//        public ComponentResource(string type, string name, Dictionary<string, object> properties = null, ResourceOptions options = default(ResourceOptions))
//        {
//            RegisterAsync(type, name, false, properties, options);
//        }
//    }
//}