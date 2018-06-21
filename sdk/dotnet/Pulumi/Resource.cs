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
        private TaskCompletionSource<string> m_UrnCompletionSource;

        public const string UnkownResourceId = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

        protected Resource()
        {
            m_UrnCompletionSource = new TaskCompletionSource<string>();
            Urn = m_UrnCompletionSource.Task;
        }

        protected virtual void OnResourceRegistrationCompete(Task<RegisterResourceResponse> resp) {
            if (resp.IsCanceled) {
                m_UrnCompletionSource.SetCanceled();
            } else if (resp.IsFaulted) {
                m_UrnCompletionSource.SetException(resp.Exception);
            } else {
                m_UrnCompletionSource.SetResult(resp.Result.Urn);
            }
        }

        public async void RegisterAsync(string type, string name, bool custom, Dictionary<string, object> properties, ResourceOptions options) {
            Serilog.Log.Debug("RegisterAsync({type}, {name})", type, name);

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

            // Kick off the registration, and when it completes, call the OnResourceRegistrationCompete method which will resolve all the tasks to their values. The fact that we don't
            // await here is by design. This method is called by child classes in their constructors, where were do not want to block.
            #pragma warning disable 4014
            Runtime.Monitor.RegisterResourceAsync(
                new RegisterResourceRequest()
                {
                    Type = type,
                    Name = name,
                    Custom = custom,
                    Protect = false,
                    Object = await SerializeProperties(properties),
                    Parent = parentUrn.Result
                }).ResponseAsync.ContinueWith((x) => OnResourceRegistrationCompete(x));;
            #pragma warning restore 4014
        }

        private async Task<Struct> SerializeProperties(Dictionary<string, object> properties) {
            if (properties == null) {
                return new Struct();
            }

            var s = new Struct();

            foreach (var kvp in properties) {
                s.Fields.Add(kvp.Key, await SerializeProperty(kvp.Value));
            }

            return s;
        }

        private async Task<Value> SerializeProperty(object o) {
            Serilog.Log.Debug("SerializeProperty({o})", o);

            var input = o as IInput;
            if (input != null) {
                // Get the ground value.
                var v = await input.GetTask();

                if (v == null) {
                    return Value.ForNull();
                }

                if (v is string) {
                    return Value.ForString((string)v);
                }

                // We marshal custom resources as strings of their provider generated IDs.
                var cr = v as CustomResource;
                if (cr != null) {
                    return Value.ForString(await cr.Id);
                }

                throw new NotImplementedException($"cannot marshal Input with underlying type ${input.GetType()}");
            }

            throw new NotImplementedException($"cannot marshal object of type ${o.GetType()}");
        }
    }
}