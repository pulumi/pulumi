using Pulumirpc;
using Pulumi;
using Pulumi.Serialization;
using Grpc.Core;
using Google.Protobuf.WellKnownTypes;
using System.Linq;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Collections.Immutable;

public static class Program
{
    class DynamicResourceProviderServicer : ResourceProvider.ResourceProviderBase
    {
        private const string Unknown = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

        private static DynamicResourceProvider GetProvider(Struct properties)
        {
            var fields = properties.Fields;

            if (!fields.TryGetValue("__provider", out var providerValue))
            {
                throw new RpcException(new Status(StatusCode.Unknown, "Dynamic resource had no '__provider' property"));
            }

            var providerString = providerValue.StringValue;

            if (providerString == null)
            {
                throw new RpcException(new Status(StatusCode.Unknown, "Dynamic resource '__provider' property was not a string"));
            }

            if (providerString == Unknown)
            {
                throw new RpcException(new Status(StatusCode.Unknown, "Dynamic resource '__provider' property was unknown"));
            }

            var pickler = new Ibasa.Pikala.Pickler();
            var memoryStream = new System.IO.MemoryStream(System.Convert.FromBase64String(providerString));
            var provider = pickler.Deserialize(memoryStream) as DynamicResourceProvider;
            if (provider == null)
            {
                throw new RpcException(new Status(StatusCode.Unknown, "Dynamic resource could not deserialise provider implementation"));
            }
            return provider;
        }

        public override Task<CheckResponse> CheckConfig(CheckRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "CheckConfig is not implemented by the dynamic provider"));
        }

        public override Task<DiffResponse> DiffConfig(DiffRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "DiffConfig is not implemented by the dynamic provider"));
        }

        public override Task<InvokeResponse> Invoke(InvokeRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "Invoke is not implemented by the dynamic provider"));
        }

        public override Task<GetSchemaResponse> GetSchema(GetSchemaRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "GetSchema is not implemented by the dynamic provider"));
        }

        public override Task<ConfigureResponse> Configure(ConfigureRequest request, ServerCallContext context)
        {
            var response = new ConfigureResponse();
            response.AcceptSecrets = false;
            return Task.FromResult(response);
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
                var provider = GetProvider(request.Properties);

                var (id, outputs) = await provider.Create(Rpc.DeserialiseProperties(request.Properties));

                var response = new CreateResponse();
                response.Id = id;
                response.Properties = Rpc.SerialiseProperties(outputs);
                response.Properties.Fields.Add("__provider",  request.Properties.Fields["__provider"]);
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
                var provider = GetProvider(request.Properties);

                var (id, outputs) = await provider.Read(request.Id, Rpc.DeserialiseProperties(request.Properties));

                var response = new ReadResponse();
                response.Id = id;
                response.Properties = Rpc.SerialiseProperties(outputs);
                response.Properties.Fields.Add("__provider",  request.Properties.Fields["__provider"]);
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
                var provider = GetProvider(request.News);
                var result = await provider.Check(Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));

                var response = new CheckResponse();
                response.Inputs = Pulumi.Serialization.Rpc.SerialiseProperties(result.Inputs);
                response.Inputs.Fields.Add("__provider",  request.News.Fields["__provider"]);

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
                var provider = GetProvider(request.News);
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
                var provider = GetProvider(request.News);
                var outputs = await provider.Update(request.Id, Rpc.DeserialiseProperties(request.Olds), Rpc.DeserialiseProperties(request.News));
                if (outputs == null)
                {
                    outputs = new Dictionary<string, object?>();
                }

                var response = new UpdateResponse();
                response.Properties = Rpc.SerialiseProperties(outputs);
                response.Properties.Fields.Add("__provider",  request.News.Fields["__provider"]);
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
                var provider = GetProvider(request.Properties);
                await provider.Delete(request.Id, Rpc.DeserialiseProperties(request.Properties));
                return new Empty();
            }
            catch (System.Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Unknown, ex.Message));
            }
        }
    }

    public static void Main(string[] args)
    {
        var monitor = new DynamicResourceProviderServicer();
        // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
        var maxRpcMessageSize = 400 * 1024 * 1024;
        var grpcChannelOptions = new List<ChannelOption> { new ChannelOption(ChannelOptions.MaxReceiveMessageLength, maxRpcMessageSize)};
        var server = new Server(grpcChannelOptions)
            {
                Services = { ResourceProvider.BindService(monitor) },
                Ports = { new ServerPort("0.0.0.0", 0, ServerCredentials.Insecure) }
            };

        server.Start();
        var port = server.Ports.First();
        System.Console.WriteLine(port.BoundPort.ToString());

        Task? shutdownTask = null;
        var exitEvent = new System.Threading.ManualResetEventSlim();
        System.Console.CancelKeyPress += (System.ConsoleCancelEventHandler)((sender, e) => {
            shutdownTask = server.ShutdownAsync();
            exitEvent.Set();
        });
        exitEvent.Wait();
        shutdownTask!.Wait();
    }
}