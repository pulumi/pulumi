using Google.Protobuf.WellKnownTypes;
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


            // Figure out the parent URN. If an explicit parent was passed in, use that. Otherwise use the global root URN. In the case where that hasn't been set yet, we must be creating
            // the ComponentResource that represents the global stack object, so pass along no parent.
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
                    Object = SerializeProperties(properties),
                    Parent = parentUrn.Result
                }
            );

            Urn = res.ResponseAsync.ContinueWith((x) => x.Result.Urn);
            return res.ResponseAsync;
        }

        private Struct SerializeProperties(Dictionary<string, object> properties) {
            if (properties == null) {
                return new Struct();
            }

            var s = new Struct();

            foreach (var kvp in properties) {
                s.Fields.Add(kvp.Key, SerializeProperty(kvp.Value));
            }

            return s;
        }

        private Value SerializeProperty(object o) {
            if (o.GetType() == typeof(string)) {
                return Value.ForString((string)o);
            }

            throw new NotImplementedException($"cannot marshal object of type ${o.GetType()}");
        }
    }
}