using Pulumirpc;
using System;
using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi
{
    public abstract class Resource
    {
        public Task<string> Urn { get; private set; }

        public const string UnkownResourceId = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

        public Resource()
        {
        }

        protected Task<RegisterResourceResponse> RegisterAsync(string type, string name, bool custom, Dictionary<string, object> properties, ResourceOptions options) {
            if (string.IsNullOrEmpty(type))
            {
                throw new ArgumentException(nameof(type));
            }

            if (string.IsNullOrEmpty(name))
            {
                throw new ArgumentException(nameof(name));
            }


            Task<string> parentUrn;

            if (options.Parent != null) {
                parentUrn = options.Parent.Urn;
            } else if (Runtime.Root != null) {
                parentUrn = Runtime.Root.Urn;
            } else {
                parentUrn = Task.FromResult("");
            }

            var res = Runtime.Monitor.RegisterResourceAsync(
                new RegisterResourceRequest()
                {
                    Type = type,
                    Name = name,
                    Custom = custom,
                    Protect = false,
                    Object = new Google.Protobuf.WellKnownTypes.Struct(),
                    Parent = parentUrn.Result
                }
            );

            Urn = res.ResponseAsync.ContinueWith((x) => x.Result.Urn);
            return res.ResponseAsync;
        }
    }
}