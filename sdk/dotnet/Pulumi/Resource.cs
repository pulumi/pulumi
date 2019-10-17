//using Google.Protobuf.Collections;
//using Google.Protobuf.WellKnownTypes;
//using Pulumirpc;
//using System;
//using System.Collections.Generic;
//using System.Threading.Tasks;

//namespace Pulumi
//{
//    public abstract class Resource
//    {
//        public Output<string> Urn { get; private set; }
//        private TaskCompletionSource<OutputState<string>> m_UrnCompletionSource;

//        public const string UnkownResourceId = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

//        protected Resource()
//        {
//            m_UrnCompletionSource = new TaskCompletionSource<OutputState<string>>();
//            Urn = new Output<string>(m_UrnCompletionSource.Task);
//        }

//        protected virtual void OnResourceRegistrationComplete(Task<RegisterResourceResponse> resp) {
//            if (resp.IsCanceled) {
//                m_UrnCompletionSource.SetCanceled();
//            } else if (resp.IsFaulted) {
//                m_UrnCompletionSource.SetException(resp.Exception);
//            } else {
//                m_UrnCompletionSource.SetResult(new OutputState<string>(resp.Result.Urn, resp.Result.Urn != null, this));
//            }
//        }

//        public async void RegisterAsync(string type, string name, bool custom, Dictionary<string, object> properties, ResourceOptions options) {
//            Serilog.Log.Debug("RegisterAsync({type}, {name})", type, name);

//            if (string.IsNullOrEmpty(type))
//            {
//                throw new ArgumentException(nameof(type));
//            }

//            if (string.IsNullOrEmpty(name))
//            {
//                throw new ArgumentException(nameof(name));
//            }

//            // Figure out the parent URN. If an explicit parent was passed in, use that. Otherwise use the global root URN. In the case where that hasn't been set yet, we must be creating
//            // the ComponentResource that represents the global stack object, so pass along no parent.
//            Task<string> parentUrn;
//            if (options.Parent == null && Runtime.Root == null) {
//                parentUrn = Task.FromResult("");
//            } else {
//                IOutput urnOutput = options.Parent?.Urn ?? Runtime.Root.Urn;
//                parentUrn = urnOutput.GetOutputStateAsync().ContinueWith(x => (string)x.Result.Value);
//            }

//            // Compute the set of dependencies this resource has. This is the union of resources the object explicitly depends on
//            // with the set of dependencies that any Output that is used as in Input has.
//            HashSet<string> dependsOnUrns = new HashSet<string>(StringComparer.Ordinal);

//            // Explicit dependencies.
//            if (options.DependsOn != null) {
//                foreach (Resource r in options.DependsOn) {
//                    dependsOnUrns.Add((string)(await ((IOutput)r.Urn).GetOutputStateAsync()).Value);
//                }
//            }

//            // Add any dependeices from any outputs that happend to be used as inputs.
//            if (properties != null) {
//                foreach (object o in properties.Values) {
//                    IInput input = o as IInput;
//                    if (input != null) {
//                        foreach (Resource r in (await input.GetValueAsOutputStateAsync()).DependsOn) {
//                            dependsOnUrns.Add((string)(await ((IOutput)r.Urn).GetOutputStateAsync()).Value);
//                        }
//                    }
//                }
//            }

//            foreach(string urn in dependsOnUrns) {
//                Serilog.Log.Debug("Dependency: {urn}", urn);
//            }

//            // Kick off the registration, and when it completes, call the OnResourceRegistrationCompete method which will resolve all the tasks to their values. The fact that we don't
//            // await here is by design. This method is called by child classes in their constructors, where were do not want to block.
//            #pragma warning disable 4014
//            RegisterResourceRequest request = new RegisterResourceRequest();
//            request.Type = type;
//            request.Name = name;
//            request.Custom = custom;
//            request.Protect = options.Protect;
//            request.Object = await SerializeProperties(properties);
//            request.Parent = await parentUrn;
//            request.Dependencies.AddRange(dependsOnUrns);
//            Runtime.Monitor.RegisterResourceAsync(request).ResponseAsync.ContinueWith((x) => OnResourceRegistrationComplete(x));
//            #pragma warning restore 4014
//        }

//        private async Task<Struct> SerializeProperties(Dictionary<string, object> properties) {
//            if (properties == null) {
//                return new Struct();
//            }

//            var s = new Struct();

//            foreach (var kvp in properties) {
//                s.Fields.Add(kvp.Key, await SerializeProperty(kvp.Value));
//            }

//            return s;
//        }

//        private async Task<Value> SerializeProperty(object o) {
//            Serilog.Log.Debug("SerializeProperty({o})", o);

//            var input = o as IInput;
//            if (input != null) {
//                OutputState<object> state = await input.GetValueAsOutputStateAsync();

//                if (!state.IsKnown) {
//                    return Value.ForString(UnkownResourceId);
//                }

//                object v = state.Value;

//                if (v == null) {
//                    return Value.ForNull();
//                }

//                if (v is string) {
//                    return Value.ForString((string)v);
//                }

//                // We marshal custom resources as strings of their provider generated IDs.
//                var cr = v as CustomResource;
//                if (cr != null) {
//                    OutputState<object> s = await ((IOutput)cr.Id).GetOutputStateAsync();
//                    return Value.ForString(s.IsKnown ? (string) s.Value : UnkownResourceId);
//                }

//                throw new NotImplementedException($"cannot marshal Input with underlying type ${input.GetType()}");
//            }

//            throw new NotImplementedException($"cannot marshal object of type ${o.GetType()}");
//        }
//    }
//}