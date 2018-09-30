using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{
    internal struct Resolver<T> {
        public TaskCompletionSource<T> Value { get; private set; }
        public TaskCompletionSource<bool> IsKnown { get; private set; }

        public static Resolver<T> Create() {
            return new Resolver<T> {
                Value = new TaskCompletionSource<T>(),
                IsKnown = new TaskCompletionSource<bool>(),
            };
        }
    }

    public abstract class Resource
    {
        internal const string UnknownId = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";
        internal static readonly Value UnknownValue = Value.ForString(UnknownId);

        public IO<string> Urn { get; private set; }

        private Dictionary<string, IO<Value>> m_outputs;
        public IReadOnlyDictionary<string, IO<Value>> Outputs { get { return m_outputs; } }

        internal Resource(string type, string name, Dictionary<string, IO<Value>> props, ResourceOptions opts = default(ResourceOptions)) {
            var resolvers = new Dictionary<string, Resolver<Value>>();
            m_outputs = new Dictionary<string, IO<Value>>();
            if (props != null) {
                foreach (var kv in props) {
                    var resolver = Resolver<Value>.Create();
                    resolvers.Add(kv.Key, resolver);
                    m_outputs.Add(kv.Key, new IO<Value>(this, resolver.Value.Task, resolver.IsKnown.Task));
                }
            }

            var urnResolver = Resolver<string>.Create();
            Urn = new IO<string>(this, urnResolver.Value.Task, urnResolver.IsKnown.Task);

            Resolver<string> idResolver = default(Resolver<string>);
            if (this is CustomResource) {
                idResolver = Resolver<string>.Create();
                var id = new IO<string>(this, idResolver.Value.Task, idResolver.IsKnown.Task);
                // Reflection magic to keep the API type safe
                var flags = System.Reflection.BindingFlags.NonPublic
                     | System.Reflection.BindingFlags.DeclaredOnly
                     | System.Reflection.BindingFlags.Instance;
                var field = typeof(CustomResource).GetField("m_id", flags);
                field.SetValue(this, id);
            }

            RegisterResourceAsync(urnResolver, idResolver, resolvers, type, name, props, opts);
        }

        private static void Resolve(Resolver<Value> resolver, Value value) {
            if(value.KindCase == Value.KindOneofCase.StringValue) {
                if(value.StringValue == Resource.UnknownId) {
                    resolver.Value.SetResult(null);
                    resolver.IsKnown.SetResult(false);
                    return;
                }
            }
            resolver.Value.SetResult(value);
            resolver.IsKnown.SetResult(true);
        }

        private async void RegisterResourceAsync(
                Resolver<string> urn, Resolver<string> id, Dictionary<string, Resolver<Value>> resolvers,
                string type, string name, Dictionary<string, IO<Value>> props, ResourceOptions opts) {
            var custom = this is CustomResource;
            var response = await Runtime.RegisterResourceAsync(type, name, custom, props, opts);

            urn.Value.SetResult(response.Urn);
            urn.IsKnown.SetResult(true);

            if (custom) {
                if(!String.IsNullOrEmpty(response.Id)) {
                    id.Value.SetResult(response.Id);
                    id.IsKnown.SetResult(true);
                } else {
                    id.Value.SetResult(null);
                    id.IsKnown.SetResult(false);
                }
            }

            foreach (var kv in response.Object.Fields) {
                if (resolvers.ContainsKey(kv.Key)) {
                    Resolve(resolvers[kv.Key], kv.Value);
                } else {
                    var io = new IO<Value>(this, Task.FromResult(kv.Value), Task.FromResult(true));
                    m_outputs.Add(kv.Key, io);
                }
            }
            foreach (var key in m_outputs.Keys) {
                if (!response.Object.Fields.ContainsKey(key)) {
                    var resolver = resolvers[key];
                    var prop = props[key];
                    if(prop != null) {
                        var output = await props[key];
                        resolver.Value.SetResult(output.Value);
                        resolver.IsKnown.SetResult(output.IsKnown);
                    } else {
                        resolver.Value.SetResult(null);
                        resolver.IsKnown.SetResult(false);
                    }
                }
            }
        }
    }
}