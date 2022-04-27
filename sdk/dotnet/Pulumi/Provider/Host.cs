using Pulumirpc;
using System;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using System.Collections.Concurrent;
using Grpc.Net.Client;

namespace Pulumi.Provider
{
    public sealed class LogMessage
    {
        /// <summary>
        /// the logging level of this message.
        /// </summary>
        public LogLevel Severity;

        /// <summary>
        /// the contents of the logged message.
        /// </summary>
        public string Message;

        /// <summary>
        /// the (optional) resource urn this log is associated with.
        /// </summary>
        public string? Urn;


        /// <summary>
        /// the (optional) stream id that a stream of log messages can be associated with. This allows
        /// clients to not have to buffer a large set of log messages that they all want to be
        /// conceptually connected.  Instead the messages can be sent as chunks (with the same stream id)
        /// and the end display can show the messages as they arrive, while still stitching them together
        /// into one total log message.
        ///
        /// 0/not-given means: do not associate with any stream.
        /// </summary>
        public int StreamId;

        /// <summary>
        /// Optional value indicating whether this is a status message.
        /// </summary>
        public bool Ephemeral;

        public LogMessage(string message)
        {
            Message = message;
        }
    }

    public interface IHost
    {
        public Task LogAsync(LogMessage message);

    }
    internal class GrpcHost : IHost
    {
        private readonly Engine.EngineClient _engine;
        // Using a static dictionary to keep track of and re-use gRPC channels
        // According to the docs (https://docs.microsoft.com/en-us/aspnet/core/grpc/performance?view=aspnetcore-6.0#reuse-grpc-channels), creating GrpcChannels is expensive so we keep track of a bunch of them here
        private static readonly ConcurrentDictionary<string, GrpcChannel> _engineChannels = new ConcurrentDictionary<string, GrpcChannel>();
        private static readonly object _channelsLock = new object();
        public GrpcHost(string engineAddress)
        {
            // Allow for insecure HTTP/2 transport (only needed for netcoreapp3.x)
            // https://docs.microsoft.com/en-us/aspnet/core/grpc/troubleshoot?view=aspnetcore-6.0#call-insecure-grpc-services-with-net-core-client
            AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);
            // maxRpcMessageSize raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
            const int maxRpcMessageSize = 400 * 1024 * 1024;
            if (_engineChannels.TryGetValue(engineAddress, out var engineChannel))
            {
                // A channel already exists for this address
                this._engine = new Engine.EngineClient(engineChannel);
            }
            else
            {
                lock (_channelsLock)
                {
                    if (_engineChannels.TryGetValue(engineAddress, out var existingChannel))
                    {
                        // A channel already exists for this address
                        this._engine = new Engine.EngineClient(existingChannel);
                    }
                    else
                    {
                        // Inititialize the engine channel once for this address
                        var channel = GrpcChannel.ForAddress(new Uri($"http://{engineAddress}"), new GrpcChannelOptions
                        {
                            MaxReceiveMessageSize = maxRpcMessageSize,
                            MaxSendMessageSize = maxRpcMessageSize,
                            Credentials = Grpc.Core.ChannelCredentials.Insecure,
                        });

                        _engineChannels[engineAddress] = channel;
                        this._engine = new Engine.EngineClient(channel);
                    }
                }
            }
        }

        public async Task LogAsync(LogMessage message)
        {
            var request = new LogRequest();
            request.Message = message.Message;
            request.Ephemeral = message.Ephemeral;
            request.Urn = message.Urn;
            request.Severity = (LogSeverity)message.Severity;
            request.StreamId = message.StreamId;
            await this._engine.LogAsync(request);
        }
    }

}
