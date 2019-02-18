using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using System.Runtime.InteropServices;
using Pulumirpc;
using Grpc.Core;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi
{
    public static class Runtime
    {
        public static string Stack { get; private set; }

        internal static Engine.EngineClient Engine { get; private set; }

        internal static ResourceMonitor.ResourceMonitorClient Monitor { get; private set; }

        public static string Project { get; private set; }

        public static bool DryRun { get; private set; }

        internal static ComponentResource Root {get; private set;}

        private static int RpcCount = 0;

        private static void EnterRpc() {
            while (true) {
                var count = RpcCount;
                if (count == -1) { while(true) { /* wait for death */ } }
                var oldCount = System.Threading.Interlocked.CompareExchange(ref RpcCount, count + 1, count);
                if(oldCount == count) {
                    break;
                }
            }
        }

        private static void ExitRpc() {
            System.Threading.Interlocked.Decrement(ref RpcCount);
        }

        private static async Task WaitForRpcs() {
            while(true) {
                var count = RpcCount;
                if (count == 0) {
                    var oldCount = System.Threading.Interlocked.CompareExchange(ref RpcCount, -1, 0);
                    if(oldCount == 0) {
                        return;
                    }
                }
                await Task.Yield();
            }
        }

        public static async Task<Struct> InvokeAsync(string tok, Struct args, InvokeOptions opts)
        {
            Log.Debug($"Invoking function: tok={tok}");
            EnterRpc();

            try {
                var request = new InvokeRequest();
                request.Tok = tok;
                request.Args = args;
                request.Provider = "";
                Log.Debug($"Invoke RPC prepared: tok={tok}; args={args}");
                var response = await Monitor.InvokeAsync(request);
                Log.Debug($"Invoke RPC finished: tok={tok}; response={response}");

                if (response.Failures.Count != 0) {
                    throw new Exception($"Invoke of '{tok}' failed: {response.Failures[0].Reason} (${response.Failures[0].Property})");
                }

                return response.Return;
            } finally {
                ExitRpc();
            }
        }

        public static async Task<RegisterResourceResponse> RegisterResourceAsync(
            string type, string name, bool custom, Dictionary<string, IO<Value>> props, ResourceOptions opts) {
            var label = $"resource:{name}[{type}]";
            Log.Debug($"Registering resource: label={label}; custom={custom}");
            EnterRpc();

            try {
                // Before we can proceed, all our dependencies must be finished.
                string[] explicitDependsOn = null;
                if (opts.DependsOn == null) {
                    explicitDependsOn = new string[0];
                } else {
                    var deps = await IO.WhenAll(opts.DependsOn.Select(res => res.Urn));
                    if (!deps.IsKnown) {
                        throw new Exception("Logical error: Explicit dependent Resource's URN unknown");
                    }
                    explicitDependsOn = deps.Value;
                }

                string parentUrn;
                if (Root == null) {
                    // Runtime.Root is only null if we're just creating it
                    parentUrn = "";
                } else if (opts.Parent == null) {
                    // If no parent was provided, parent to the root resource.
                    var urn = await Root.Urn;
                    if (!urn.IsKnown) {
                        throw new Exception("Logical error: Root Resource's URN unknown");
                    }
                    parentUrn = urn.Value;
                } else {
                    var urn = await opts.Parent.Urn;
                    if (!urn.IsKnown) {
                        throw new Exception("Logical error: Parent Resource's URN unknown");
                    }
                    parentUrn = urn.Value;
                }

                if (props == null) {
                    props = new Dictionary<string, IO<Value>>();
                }

                var dependsOn = new HashSet<string>();
                dependsOn.UnionWith(explicitDependsOn);
                foreach (var prop in props) {
                    if (prop.Value != null) {
                        var deps = await IO.WhenAll(
                            prop.Value.Resources.Select(resource => resource.Urn));
                        if (!deps.IsKnown) {
                            throw new Exception("Logical error: Implicit dependent Resource's URN unknown");
                        }
                        dependsOn.UnionWith(deps.Value);
                    }
                }

                var obj = new Struct();
                foreach (var prop in props) {
                    if (prop.Value != null) {
                        var property = await prop.Value;
                        if (!property.IsKnown) {
                            obj.Fields[prop.Key] = Resource.UnknownValue;
                        }
                        else {
                            obj.Fields[prop.Key] = property.Value;
                        }
                    }
                }

                var request = new RegisterResourceRequest();
                request.Type = type;
                request.Name = name;
                request.Custom = custom;
                request.Parent = parentUrn;
                request.Protect = opts.Protect;
                request.Provider = "";
                request.Dependencies.AddRange(dependsOn);
                request.Object = obj;

                Log.Debug($"RegisterResource RPC prepared: label={label}; request={request}");
                var response = await Monitor.RegisterResourceAsync(request);
                Log.Debug($"RegisterResource RPC finished: label={label}; response={response}");

                return response;
            } finally {
                ExitRpc();
            }
        }

        internal static async Task RegisterResourceOutputsAsync(Resource res, Dictionary<string, IO<Value>> outputs) {
            EnterRpc();
            try {
                // The registration could very well still be taking place, so we will need to wait for its URN.
                // Additionally, the output properties might have come from other resources, so we must await those too.
                var urn = await res.Urn;
                if(!urn.IsKnown) {
                    throw new Exception("Logical error: Output Resource's URN unknown");
                }
                var obj = new Struct();
                foreach (var output in outputs) {
                    if (output.Value != null) {
                        var property = await output.Value;
                        if (!property.IsKnown) {
                            obj.Fields[output.Key] = Resource.UnknownValue;
                        }
                        else if (property.Value.KindCase != Value.KindOneofCase.NullValue) {
                            obj.Fields[output.Key] = property.Value;
                        }
                    }
                }

                Log.Debug($"RegisterResourceOutputs RPC prepared: urn={urn.Value}; object={obj}");
                var request = new RegisterResourceOutputsRequest();
                request.Urn = urn.Value;
                request.Outputs = obj;
                await Runtime.Monitor.RegisterResourceOutputsAsync(request);
                Log.Debug($"RegisterResourceOutputs RPC finished: urn={urn.Value}");
            } finally {
                ExitRpc();
            }
        }

        private static Dictionary<string, IO<Value>> Exports = new Dictionary<string, IO<Value>>();

        public static void Export(string key, IO<string> value) {
            Exports.Add(key, value.Select(v => Value.ForString(v)));
        }

        // TODO(ellismg): Perhaps we should have another overload Run<T>(Func<T> f) and we use reflection over the T
        // to get all public fields and properties of type Input<T> and set them as outputs?
        private static async Task RunAsync(Action run) {
            // TODO(ellismg): any one of these could be null, and we need to guard against that for ones that must
            // be set (I don't know the set off the top of my head.  I think that everything except tracing is
            // required.  Also, they could be bad values (e.g. parallel may not be something that can be `bool.Parsed`
            // and we'd like to fail in a nicer manner.
            string monitor = Environment.GetEnvironmentVariable("PULUMI_MONITOR");
            string engine = Environment.GetEnvironmentVariable("PULUMI_ENGINE");
            string project = Environment.GetEnvironmentVariable("PULUMI_PROJECT");
            string stack = Environment.GetEnvironmentVariable("PULUMI_STACK");
            string pwd = Environment.GetEnvironmentVariable("PULUMI_PWD");
            string dryRun = Environment.GetEnvironmentVariable("PULUMI_DRY_RUN");
            string parallel = Environment.GetEnvironmentVariable("PULUMI_PARALLEL");
            string tracing = Environment.GetEnvironmentVariable("PULUMI_TRACING");

            Channel engineChannel = new Channel(engine, ChannelCredentials.Insecure);
            Channel monitorChannel = new Channel(monitor, ChannelCredentials.Insecure);

            Engine = new Engine.EngineClient(engineChannel);
            Monitor = new ResourceMonitor.ResourceMonitorClient(monitorChannel);
            Project = project;
            Stack = stack;
            DryRun = bool.Parse(dryRun);

            Console.WriteLine($"Running with \U0001F379 on {RuntimeInformation.FrameworkDescription} on {RuntimeInformation.OSDescription}");

            TaskScheduler.UnobservedTaskException += new EventHandler<UnobservedTaskExceptionEventArgs>((obj, args) => {
                foreach(var exception in args.Exception.InnerExceptions) {
                    var runError = exception as RunError;
                    if (runError != null) {
                        Log.Error(runError.Message);
                    } else {
                        Log.Error("Running program failed with an unhandled exception:");
                        Log.Error(exception.ToString());
                    }
                }
                args.SetObserved();
            });

            try {
                var root = new ComponentResource("pulumi:pulumi:Stack", $"{Runtime.Project}-{Runtime.Stack}");
                // Wait for root to be registered then set the global value and run the stack
                var rootUrn = await root.Urn;
                if (!rootUrn.IsKnown) {
                    throw new Exception("Logical error: Root Resource's URN unknown");
                }
                Root = root;
                run();

                // Await and wrap up all the exports
                var exportTask = RegisterResourceOutputsAsync(root, Exports);
                while (!exportTask.IsCompleted) {
                    // Run GC collects here to force any loose tasks to be collected and throw their errors
                    GC.Collect();
                    GC.WaitForPendingFinalizers();
                    GC.Collect();
                    await Task.Delay(1);
                }
            } catch (RunError err) {
                // For errors that are subtypes of RunError, we will print the message without hitting the unhandled error
                // logic, which will dump all sorts of verbose spew like the origin source and stack trace.
                Log.Error(err.Message);
            } catch (Exception err) {
                Log.Error("Running program failed with an unhandled exception:");
                Log.Error(err.ToString());
            } finally {
                // Wait for all RPCs to finish
                await WaitForRpcs();
                await monitorChannel.ShutdownAsync();
                await engineChannel.ShutdownAsync();
            }
        }

        public static void Run(Action run) {
            RunAsync(run).Wait();
        }
    }
}