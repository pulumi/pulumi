using System;

namespace Pulumi {
    /// <summary>
    /// ResourceOptions is a bag of optional settings that control a resource's behavior.
    /// </summary>
    public struct ResourceOptions {
        public static ResourceOptions None = default(ResourceOptions);

        public Resource Parent {get; set;}
        public Resource[] DependsOn {get; set;}
        public bool Protect {get; set;}

        public IO<string> Id {get; set;}

        public ResourceOptions WithParent(Resource parent) {
            var n = this;
            n.Parent = parent;
            return n;
        }
    }
}