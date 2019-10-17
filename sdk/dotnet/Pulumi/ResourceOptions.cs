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