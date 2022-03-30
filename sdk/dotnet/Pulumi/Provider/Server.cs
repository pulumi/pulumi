using Pulumirpc;
using Pulumi.Serialization;
using Google.Protobuf.WellKnownTypes;
using System.Linq;
using System.Threading.Tasks;
using System.Collections.Generic;
using Grpc.Core;
using System;

using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using System.Net;
using System.Diagnostics;

namespace Pulumi.Provider
{
    class ResourceProviderService : ResourceProvider.ResourceProviderBase
    {
        private readonly IProvider provider;
        public ResourceProviderService(IProvider provider)
        {
            this.provider = provider;
        }

        public override async Task<CheckResponse> CheckConfig(CheckRequest request, ServerCallContext context)
        {
            try
            {
                var result = await provider.CheckConfig(Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));

                var response = new CheckResponse();
                response.Inputs = Pulumi.Serialization.Rpc.SerialiseProperties(result.Inputs);

                if (result.Failures != null)
                {
                    foreach(var failure in result.Failures)
                    {
                        var protoFailure = new Pulumirpc.CheckFailure();
                        protoFailure.Property = failure.Property;
                        protoFailure.Reason = failure.Reason;
                        response.Failures.Add(protoFailure);
                    }
                }

                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<DiffResponse> DiffConfig(DiffRequest request, ServerCallContext context)
        {
            try
            {
                var result = await provider.DiffConfig(request.Id, Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));
                var response = new DiffResponse();
                if (result.Changes.HasValue)
                {
                    if (result.Changes.Value)
                    {
                        response.Changes = DiffResponse.Types.DiffChanges.DiffSome;
                    }
                    else
                    {
                        response.Changes = DiffResponse.Types.DiffChanges.DiffNone;
                    }
                }
                else
                {
                    response.Changes = DiffResponse.Types.DiffChanges.DiffUnknown;
                }
                if (result.Replaces != null) {
                    response.Replaces.AddRange(result.Replaces);
                }
                if (result.Stables != null) {
                    response.Stables.AddRange(result.Stables);
                }
                response.DeleteBeforeReplace = result.DeleteBeforeReplace;
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<InvokeResponse> Invoke(InvokeRequest request, ServerCallContext context)
        {
            try
            {
                var result = await provider.Invoke(request.Tok, Rpc.DeserialiseProperties(request.Args));

                var response = new InvokeResponse();
                response.Return = Pulumi.Serialization.Rpc.SerialiseProperties(result.Return);

                if (result.Failures != null)
                {
                    foreach(var failure in result.Failures)
                    {
                        var protoFailure = new Pulumirpc.CheckFailure();
                        protoFailure.Property = failure.Property;
                        protoFailure.Reason = failure.Reason;
                        response.Failures.Add(protoFailure);
                    }
                }

                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<GetSchemaResponse> GetSchema(GetSchemaRequest request, ServerCallContext context)
        {
            try
            {
                var response = new GetSchemaResponse();
                response.Schema = await provider.GetSchema(request.Version);
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<ConfigureResponse> Configure(ConfigureRequest request, ServerCallContext context)
        {
            try
            {
                var vars = new Dictionary<string, string>();
                foreach(var kv in request.Variables){
                    vars.Add(kv.Key, kv.Value);
                }

                var result = await provider.Configure(request.AcceptSecrets, request.AcceptResources, Rpc.DeserialiseProperties(request.Args), vars);
                var response = new ConfigureResponse();
                response.AcceptOutputs = result.AcceptOutputs;
                response.AcceptResources = result.AcceptResources;
                response.AcceptSecrets = result.AcceptSecrets;
                response.SupportsPreview = result.SupportsPreview;
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override Task<PluginInfo> GetPluginInfo(Empty request, ServerCallContext context)
        {
            var response = new PluginInfo();
            response.Version = "0.1.0";
            return Task.FromResult(response);
        }

        public override Task<Empty> Cancel(Empty request, ServerCallContext context)
        {
            return Task.FromResult(new Empty());
        }

        public override async Task<CreateResponse> Create(CreateRequest request, ServerCallContext context)
        {
            try
            {
                var (id, outputs) = await provider.Create(Rpc.DeserialiseProperties(request.Properties));

                var response = new CreateResponse();
                response.Id = id;
                response.Properties = Rpc.SerialiseProperties(outputs);
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<ReadResponse> Read(ReadRequest request, ServerCallContext context)
        {
            try
            {
                var (id, outputs) = await provider.Read(request.Id, Rpc.DeserialiseProperties(request.Properties));

                var response = new ReadResponse();
                response.Id = id;
                response.Properties = Rpc.SerialiseProperties(outputs);
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<CheckResponse> Check(CheckRequest request, ServerCallContext context)
        {
            try
            {
                var result = await provider.Check(Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));

                var response = new CheckResponse();
                response.Inputs = Pulumi.Serialization.Rpc.SerialiseProperties(result.Inputs);

                if (result.Failures != null)
                {
                    foreach(var failure in result.Failures)
                    {
                        var protoFailure = new Pulumirpc.CheckFailure();
                        protoFailure.Property = failure.Property;
                        protoFailure.Reason = failure.Reason;
                        response.Failures.Add(protoFailure);
                    }
                }

                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<DiffResponse> Diff(DiffRequest request, ServerCallContext context)
        {
            try
            {
                var result = await provider.Diff(request.Id, Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));
                var response = new DiffResponse();
                if (result.Changes.HasValue)
                {
                    if (result.Changes.Value)
                    {
                        response.Changes = DiffResponse.Types.DiffChanges.DiffSome;
                    }
                    else
                    {
                        response.Changes = DiffResponse.Types.DiffChanges.DiffNone;
                    }
                }
                else
                {
                    response.Changes = DiffResponse.Types.DiffChanges.DiffUnknown;
                }
                if (result.Replaces != null) {
                    response.Replaces.AddRange(result.Replaces);
                }
                if (result.Stables != null) {
                    response.Stables.AddRange(result.Stables);
                }
                response.DeleteBeforeReplace = result.DeleteBeforeReplace;
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<UpdateResponse> Update(UpdateRequest request, ServerCallContext context)
        {
            try
            {
                var outputs = await provider.Update(request.Id, Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));
                if (outputs == null)
                {
                    outputs = new Dictionary<string, object?>();
                }

                var response = new UpdateResponse();
                response.Properties = Rpc.SerialiseProperties(outputs);
                return response;
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }

        public override async Task<Empty> Delete(DeleteRequest request, ServerCallContext context)
        {
            try
            {
                await provider.Delete(request.Id, Rpc.DeserialiseProperties(request.Properties));
                return new Empty();
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }
    }

    public static class Server {
        public static async Task Main(IProvider provider, string[] args, System.Threading.CancellationToken cancellationToken)
        {
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            var maxRpcMessageSize = 400 * 1024 * 1024;

            var host = Host.CreateDefaultBuilder()
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    webBuilder
                        .ConfigureKestrel(kestrelOptions =>
                        {
                            kestrelOptions.Listen(IPAddress.Any, 0, listenOptions =>
                            {
                                listenOptions.Protocols = HttpProtocols.Http2;
                            });
                        })
                        .ConfigureAppConfiguration((context, config) =>
                        {
                            // clear so we don't read appsettings.json
                            // note that we also won't read environment variables for config
                            config.Sources.Clear();
                        })
                        .ConfigureLogging(loggingBuilder =>
                        {
                            // disable default logging
                            loggingBuilder.ClearProviders();
                        })
                        .ConfigureServices(services =>
                        {
                                // to be injected into ResourceProviderService
                            services.AddSingleton<IProvider>(provider);

                            services.AddGrpc(grpcOptions =>
                            {
                                grpcOptions.MaxReceiveMessageSize = maxRpcMessageSize;
                                grpcOptions.MaxSendMessageSize = maxRpcMessageSize;
                            });
                        })
                        .Configure(app =>
                        {
                            app.UseRouting();
                            app.UseEndpoints(endpoints =>
                            {
                                endpoints.MapGrpcService<ResourceProviderService>();
                            });
                        });
                })
                .Build();

            // before starting the host, set up this callback to tell us what port was selected
            var portTcs = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
            var portRegistration = host.Services.GetRequiredService<IHostApplicationLifetime>().ApplicationStarted.Register(() =>
            {
                try
                {
                    var serverFeatures = host.Services.GetRequiredService<IServer>().Features;
                    var addresses = serverFeatures.Get<IServerAddressesFeature>().Addresses.ToList();
                    Debug.Assert(addresses.Count == 1, "Server should only be listening on one address");
                    var uri = new Uri(addresses[0]);
                    portTcs.TrySetResult(uri.Port);
                }
                catch (Exception ex)
                {
                    portTcs.TrySetException(ex);
                }
            });

            await host.StartAsync(cancellationToken);

            var port = await portTcs.Task;
            System.Console.WriteLine(port.ToString());

            var exitTcs = new TaskCompletionSource<Task>(TaskCreationOptions.RunContinuationsAsynchronously);
            var registration = cancellationToken.Register(() => {
                exitTcs.SetResult(host.StopAsync());
            });

            var stopTask = await exitTcs.Task;
            await stopTask;
        }
    }
}

// Example Program.cs
//
//public class Provider : IProvider
//{
//    public virtual Task<(string, IDictionary<string, object?>)> Create(ImmutableDictionary<string, object?> properties)
//    {
//        return ("id", properties);
//    }
//}
//
//public static class Program {
//    public static void Main(string[] args) {
//        var cts = new System.Threading.CancellationTokenSource();
//        Console.CancelKeyPress += (object sender, ConsoleCancelEventArgs e) => cts.Cancel()
//        var provider = new Provider();
//
//        Pulumi.Provider.Server.Main(provider, args, cts.Token)
//    }
//
//}