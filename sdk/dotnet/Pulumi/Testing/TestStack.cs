using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi.Testing
{
    /// <summary>
    /// TestStack is used internally to implement Deployment.TestAsync overload where the user has only a function
    /// that creates resources, not a stack definition. This Stack is filtered out from the created resources
    /// and it is used to obtain the outputs from the function that created the resources, if any.
    /// </summary>
    internal class TestStack : Stack
    {
        public TestStack(Action createResources)
        {
            createResources();
            Outputs = new Dictionary<string, object?>();
        }
        
        public TestStack(Func<Task> createResources)
        {
            createResources().GetAwaiter().GetResult();
            Outputs = new Dictionary<string, object?>();
        }
        
        public TestStack(Func<IDictionary<string, object?>> createResources)
        {
            Outputs = createResources();
        }
        
        public TestStack(Func<Task<IDictionary<string, object?>>> createResources)
        {
            Outputs = createResources().GetAwaiter().GetResult();
        }
        
        public new IDictionary<string, object?> Outputs { get; set;  }
    }
}
