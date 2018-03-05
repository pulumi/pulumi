using Pulumirpc;
using System;
using System.Collections.Generic;

namespace Pulumi
{
    public abstract class Resource
    {
        public string Urn { get; private set; }

        public const string UnkownResourceId = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

        public Resource()
        {
        }

        protected RegisterResourceResponse Register(string type, string name, bool custom, Dictionary<string, object> properties, ResourceOptions options) {
            if (string.IsNullOrEmpty(type))
            {
                throw new ArgumentException(nameof(type));
            }

            if (string.IsNullOrEmpty(name))
            {
                throw new ArgumentException(nameof(name));
            }

            RegisterResourceResponse res = Runtime.Monitor.RegisterResource(
                new RegisterResourceRequest()
                {
                    Type = type,
                    Name = name,
                    Custom = custom,
                    Protect = false,
                    Object = new Google.Protobuf.WellKnownTypes.Struct(),
                    Parent = options.Parent?.Urn ?? Runtime.Root?.Urn ?? "",
                });

            Urn = res.Urn;

            return res;
        }
    }
}