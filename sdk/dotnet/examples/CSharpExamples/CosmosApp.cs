using System.Collections.Generic;

using Pulumi;
using Pulumi.Azure.Core;

namespace CSharpExamples
{
    public class CosmosAppAci
    {
        public static IDictionary<string, Output<string>> Run()
        {
            var resourceGroup = new ResourceGroup("rg");

            return new Dictionary<string, Output<string>>
            {
            };
        }
    }
}
