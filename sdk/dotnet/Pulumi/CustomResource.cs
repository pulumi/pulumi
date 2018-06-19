using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi {

    public class CustomResource : Resource {

        protected Task<Pulumirpc.RegisterResourceResponse> m_registrationResponse;
        public Task<string> Id { get; private set;}
        public CustomResource(string type, string name, Dictionary<string, object> properties = null, ResourceOptions options = default(ResourceOptions))  {
            m_registrationResponse = RegisterAsync(type, name, true, properties, options);

            Id = m_registrationResponse.ContinueWith((x) => {
                return x.Result.Id;
            });
        }
    }
}