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
    public struct ResourceOptions
    {
    }
}

//using System;

//namespace Pulumi {
//    public struct ResourceOptions {
//        public static ResourceOptions None = default(ResourceOptions);

//        public Resource Parent {get; set;}
//        public Resource[] DependsOn {get; set;}
//        public bool Protect {get; set;}

//        public ResourceOptions WithParent(Resource parent) {
//            var n = this;
//            n.Parent = parent;
//            return n;
//        }        
//    }
//}