using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi {

    public class CustomResource : Resource {
        public Task<string> Id { get; private set;}
        public CustomResource(string type, string name, Dictionary<string, object> properties = null, ResourceOptions options = default(ResourceOptions))  {
            var res = RegisterAsync(type, name, true, properties, options);

            Id = res.ContinueWith((x) => {
                return x.Result.Id;
            });
        }
    }
}