using System;

namespace Pulumi {
    /// <summary>
    /// InvokeOptions is a bag of options that control the behavior of a call to Runtime.InvokeAsync.
    /// </summary>
    public struct InvokeOptions {
        public static InvokeOptions None = default(InvokeOptions);

        /// <summary>
        /// An optional parent to use for default options for this invoke (e.g. the default provider to use).
        /// </summary>
        public Resource Parent {get; set;}

        public InvokeOptions WithParent(Resource parent) {
            var n = this;
            n.Parent = parent;
            return n;
        }
    }
}