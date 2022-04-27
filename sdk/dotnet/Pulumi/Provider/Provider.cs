using Pulumirpc;
using Pulumi.Serialization;
using Grpc.Core;
using Google.Protobuf.WellKnownTypes;
using System;
using System.Linq;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Collections.Immutable;
using Microsoft.Extensions.Hosting;
using Microsoft.AspNetCore.Hosting;
using System.Net;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using System.Diagnostics;
using System.Threading;
using Microsoft.Extensions.Configuration;

namespace Pulumi.Provider
{
    public readonly struct ResourceReference
    {
        public readonly string URN;
        public readonly PropertyValue ID;
        public readonly string PackageVersion;

        public ResourceReference(string urn, PropertyValue id, string version)
        {
            URN = urn;
            ID = id;
            PackageVersion = version;
        }
    }

    public readonly struct OutputReference
    {
        public readonly PropertyValue? Value;
        public readonly ImmutableArray<string> Dependencies;

        public OutputReference(PropertyValue? value, ImmutableArray<string> dependencies)
        {
            Value = value;
            Dependencies = dependencies;
        }
    }

    public sealed class PropertyValue
    {
        private bool IsNull
        {
            get
            {
                // Null if all the other properties aren't set
                return
                    BoolValue == null &&
                    NumberValue == null &&
                    StringValue == null &&
                    ArrayValue == null &&
                    ObjectValue == null &&
                    AssetValue == null &&
                    ArchiveValue == null &&
                    SecretValue == null &&
                    ResourceValue == null &&
                    !IsComputed;
            }
        }

        private readonly bool? BoolValue;
        private readonly double? NumberValue;
        private readonly string? StringValue;
        private readonly ImmutableArray<PropertyValue>? ArrayValue;
        private readonly ImmutableDictionary<string, PropertyValue>? ObjectValue;
        private readonly Asset? AssetValue;
        private readonly Archive? ArchiveValue;
        private readonly PropertyValue? SecretValue;
        private readonly ResourceReference? ResourceValue;
        private readonly OutputReference? OutputValue;
        private readonly bool IsComputed;

        public T Match<T>(
            Func<T> nullCase,
            Func<bool, T> boolCase,
            Func<double, T> numberCase,
            Func<string, T> stringCase,
            Func<ImmutableArray<PropertyValue>, T> arrayCase,
            Func<ImmutableDictionary<string, PropertyValue>, T> objectCase,
            Func<Asset, T> assetCase,
            Func<Archive, T> archiveCase,
            Func<PropertyValue, T> secretCase,
            Func<ResourceReference, T> resourceCase,
            Func<OutputReference, T> outputCase,
            Func<T> computedCase)
        {
            if (BoolValue != null) return boolCase(BoolValue.Value);
            if (NumberValue != null) return numberCase(NumberValue.Value);
            if (StringValue != null) return stringCase(StringValue);
            if (ArrayValue != null) return arrayCase(ArrayValue.Value);
            if (ObjectValue != null) return objectCase(ObjectValue);
            if (AssetValue != null) return assetCase(AssetValue);
            if (ArchiveValue != null) return archiveCase(ArchiveValue);
            if (SecretValue != null) return secretCase(SecretValue);
            if (ResourceValue != null) return resourceCase(ResourceValue.Value);
            if (OutputValue != null) return outputCase(OutputValue.Value);
            if (IsComputed) return computedCase();
            return nullCase();
        }

        private enum SpecialType
        {
            IsNull,
            IsComputed,
        }

        public static PropertyValue Null = new PropertyValue(SpecialType.IsComputed);
        public static PropertyValue Computed = new PropertyValue(SpecialType.IsNull);

        public PropertyValue(bool value)
        {
            BoolValue = value;
        }
        public PropertyValue(double value)
        {
            NumberValue = value;
        }
        public PropertyValue(string value)
        {
            StringValue = value;
        }
        public PropertyValue(ImmutableArray<PropertyValue> value)
        {
            ArrayValue = value;
        }
        public PropertyValue(ImmutableDictionary<string, PropertyValue> value)
        {
            ObjectValue = value;
        }
        public PropertyValue(Asset value)
        {
            AssetValue = value;
        }
        public PropertyValue(Archive value)
        {
            ArchiveValue = value;
        }
        public PropertyValue(PropertyValue value)
        {
            SecretValue = value;
        }
        public PropertyValue(ResourceReference value)
        {
            ResourceValue = value;
        }
        public PropertyValue(OutputReference value)
        {
            OutputValue = value;
        }

        private PropertyValue(SpecialType type)
        {
            if (type == SpecialType.IsComputed)
            {
                IsComputed = true;
            }
        }

        static bool TryGetStringValue(Google.Protobuf.Collections.MapField<string, Value> fields, string key, [System.Diagnostics.CodeAnalysis.NotNullWhen(true)] out string? result)
        {
            if (fields.TryGetValue(key, out var value) && value.KindCase == Value.KindOneofCase.StringValue)
            {
                result = value.StringValue;
                return true;
            }
            result = null;
            return false;
        }

        internal static ImmutableDictionary<string, PropertyValue> Marshal(Struct properties)
        {
            var builder = ImmutableDictionary.CreateBuilder<string, PropertyValue>();
            foreach (var item in properties.Fields)
            {
                builder.Add(item.Key, Marshal(item.Value));
            }
            return builder.ToImmutable();
        }

        internal static PropertyValue Marshal(Value value)
        {
            switch (value.KindCase)
            {
                case Value.KindOneofCase.NullValue:
                    return PropertyValue.Null;
                case Value.KindOneofCase.BoolValue:
                    return new PropertyValue(value.BoolValue);
                case Value.KindOneofCase.NumberValue:
                    return new PropertyValue(value.NumberValue);
                case Value.KindOneofCase.StringValue:
                {
                    // This could be the special unknown value
                    if (value.StringValue == Constants.UnknownValue)
                    {
                        return PropertyValue.Computed;
                    }
                    return new PropertyValue(value.StringValue);
                }
                case Value.KindOneofCase.ListValue:
                {
                    var listValue = value.ListValue;
                    var builder = ImmutableArray.CreateBuilder<PropertyValue>(listValue.Values.Count);
                    foreach (var item in listValue.Values)
                    {
                        builder.Add(Marshal(item));
                    }
                    return new PropertyValue(builder.ToImmutable());
                }
                case Value.KindOneofCase.StructValue:
                {
                    // This could be a plain object, or one of our specials
                    var structValue = value.StructValue;

                    if (TryGetStringValue(structValue.Fields, Constants.SpecialSigKey, out var sig))
                    {
                        switch (sig)
                        {
                            case Constants.SpecialSecretSig:
                            {
                                if (!structValue.Fields.TryGetValue(Constants.ValueName, out var secretValue))
                                    throw new InvalidOperationException("Secrets must have a field called 'value'");

                                return new PropertyValue(Marshal(secretValue));
                            }
                            case Constants.SpecialAssetSig:
                            {
                                if (TryGetStringValue(structValue.Fields, Constants.AssetOrArchivePathName, out var path))
                                    return new PropertyValue(new FileAsset(path));

                                if (TryGetStringValue(structValue.Fields, Constants.AssetOrArchiveUriName, out var uri))
                                    return new PropertyValue(new RemoteAsset(uri));

                                if (TryGetStringValue(structValue.Fields, Constants.AssetTextName, out var text))
                                    return new PropertyValue(new StringAsset(text));

                                throw new InvalidOperationException("Value was marked as Asset, but did not conform to required shape.");
                            }
                            case Constants.SpecialArchiveSig:
                            {
                                if (TryGetStringValue(structValue.Fields, Constants.AssetOrArchivePathName, out var path))
                                    return new PropertyValue(new FileArchive(path));

                                if (TryGetStringValue(structValue.Fields, Constants.AssetOrArchiveUriName, out var uri))
                                    return new PropertyValue(new RemoteArchive(uri));

                                if (structValue.Fields.TryGetValue(Constants.ArchiveAssetsName, out var assetsValue))
                                {
                                    if (assetsValue.KindCase == Value.KindOneofCase.StructValue)
                                    {
                                        var assets = ImmutableDictionary.CreateBuilder<string, AssetOrArchive>();
                                        foreach (var (name, val) in assetsValue.StructValue.Fields)
                                        {
                                            var innerAssetOrArchive = Marshal(val);
                                            if (innerAssetOrArchive.AssetValue != null)
                                            {
                                                assets[name] = innerAssetOrArchive.AssetValue;
                                            }
                                            else if (innerAssetOrArchive.ArchiveValue != null)
                                            {
                                                assets[name] = innerAssetOrArchive.ArchiveValue;
                                            }
                                            else
                                            {
                                                throw new InvalidOperationException("AssetArchive contained an element that wasn't itself an Asset or Archive.");
                                            }
                                        }

                                        return new PropertyValue(new AssetArchive(assets.ToImmutable()));
                                    }
                                }

                                throw new InvalidOperationException("Value was marked as Archive, but did not conform to required shape.");
                            }
                            case Constants.SpecialResourceSig:
                            {
                                if (!TryGetStringValue(structValue.Fields, Constants.UrnPropertyName, out var urn))
                                {
                                    throw new InvalidOperationException("Value was marked as a Resource, but did not conform to required shape.");
                                }

                                if (!TryGetStringValue(structValue.Fields, Constants.ResourceVersionName, out var version))
                                {
                                    version = "";
                                }

                                if (!structValue.Fields.TryGetValue(Constants.IdPropertyName, out var id))
                                {
                                    throw new InvalidOperationException("Value was marked as a Resource, but did not conform to required shape.");
                                }

                                return new PropertyValue(new ResourceReference(urn, Marshal(id), version));
                            }
                            case Constants.SpecialOutputValueSig:
                            {
                                PropertyValue? element = null;
                                if (structValue.Fields.TryGetValue(Constants.ValueName, out var knownElement))
                                {
                                    element = Marshal(knownElement);
                                }
                                var secret = false;
                                if (structValue.Fields.TryGetValue(Constants.SecretName, out var v))
                                {
                                    if (v.KindCase == Value.KindOneofCase.BoolValue)
                                    {
                                        secret = v.BoolValue;
                                    }
                                    else
                                    {
                                        throw new InvalidOperationException("Value was marked as an Output, but did not conform to required shape.");
                                    }
                                }

                                var dependenciesBuilder = ImmutableArray.CreateBuilder<string>();
                                if (structValue.Fields.TryGetValue(Constants.DependenciesName, out var dependencies))
                                {
                                    if (dependencies.KindCase == Value.KindOneofCase.ListValue)
                                    {
                                        foreach (var dependency in dependencies.ListValue.Values)
                                        {
                                            if (dependency.KindCase == Value.KindOneofCase.StringValue)
                                            {
                                                dependenciesBuilder.Add(dependency.StringValue);
                                            }
                                            else
                                            {
                                                throw new InvalidOperationException("Value was marked as an Output, but did not conform to required shape.");
                                            }
                                        }
                                    }
                                    else
                                    {
                                        throw new InvalidOperationException("Value was marked as an Output, but did not conform to required shape.");
                                    }
                                }

                                var output = new OutputReference(element, dependenciesBuilder.ToImmutable());

                                if (secret)
                                {
                                    return new PropertyValue(new PropertyValue(output));
                                }
                                else
                                {
                                    return new PropertyValue(output);
                                }
                            }

                            default:
                                throw new InvalidOperationException($"Unrecognized special signature: {sig}");
                        }
                    }
                    else
                    {
                        // Just a plain object
                        var builder = ImmutableDictionary.CreateBuilder<string, PropertyValue>();
                        foreach (var item in structValue.Fields)
                        {
                            builder.Add(item.Key, Marshal(item.Value));
                        }
                        return new PropertyValue(builder.ToImmutable());
                    }
                }

                case Value.KindOneofCase.None:
                default:
                    throw new InvalidOperationException($"Unexpected grpc value: {value}");
            }
        }

        internal static Struct Unmarshal(IDictionary<string, PropertyValue> properties)
        {
            var result = new Struct();
            foreach (var item in properties)
            {
                result.Fields[item.Key] = Unmarshal(item.Value);
            }
            return result;
        }

        private static Value UnmarshalAsset(Asset asset)
        {
            var result = new Struct();
            result.Fields[Constants.SpecialSigKey] = Value.ForString(Constants.SpecialAssetSig);
            result.Fields[asset.PropName] = Value.ForString((string)asset.Value);
            return Value.ForStruct(result);
        }

        private static Value UnmarshalArchive(Archive archive)
        {
            var result = new Struct();
            result.Fields[Constants.SpecialSigKey] = Value.ForString(Constants.SpecialAssetSig);

            if (archive.Value is string str)
            {
                result.Fields[archive.PropName] = Value.ForString(str);
            }
            else
            {
                var inner = (ImmutableDictionary<string, AssetOrArchive>)archive.Value;
                var innerStruct = new Struct();
                foreach (var item in inner)
                {
                    innerStruct.Fields[item.Key] = UnmarshalAssetOrArchive(item.Value);
                }
                result.Fields[archive.PropName] = Value.ForStruct(innerStruct);
            }
            return Value.ForStruct(result);
        }

        private static Value UnmarshalAssetOrArchive(AssetOrArchive assetOrArchive)
        {
            if (assetOrArchive is Asset asset)
            {
                return UnmarshalAsset(asset);
            }
            else if (assetOrArchive is Archive archive)
            {
                return UnmarshalArchive(archive);
            }
            throw new InvalidOperationException("Internal error, AssetOrArchive was neither an Asset or Archive");
        }

        private static Value UnmarshalOutput(OutputReference output, bool secret)
        {
            var result = new Struct();
            result.Fields[Constants.SpecialSigKey] = Value.ForString(Constants.SpecialOutputValueSig);
            if (output.Value != null)
            {
                result.Fields[Constants.ValueName] = Unmarshal(output.Value);
            }

            var dependencies = new Value[output.Dependencies.Length];
            var i = 0;
            foreach (var dependency in output.Dependencies)
            {
                dependencies[i++] = Value.ForString(dependency);
            }
            result.Fields[Constants.DependenciesName] = Value.ForList(dependencies);
            result.Fields[Constants.SecretName] = Value.ForBool(secret);

            return Value.ForStruct(result);
        }

        internal static Value Unmarshal(PropertyValue value)
        {
            return value.Match<Value>(
                () => Value.ForNull(),
                b => Value.ForBool(b),
                n => Value.ForNumber(n),
                s => Value.ForString(s),
                a =>
                {
                    var result = new Value[a.Length];
                    for (int i = 0; i < a.Length; ++i)
                    {
                        result[i] = Unmarshal(a[i]);
                    }
                    return Value.ForList(result);
                },
                o =>
                {
                    var result = new Struct();
                    foreach (var item in o)
                    {
                        result.Fields[item.Key] = Unmarshal(item.Value);
                    }
                    return Value.ForStruct(result);
                },
                asset => UnmarshalAsset(asset),
                archive => UnmarshalArchive(archive),
                secret =>
                {
                    // Special case if our secret value is an output
                    if (secret.OutputValue != null)
                    {
                        return UnmarshalOutput(secret.OutputValue.Value, true);
                    }
                    var result = new Struct();
                    result.Fields[Constants.SpecialSigKey] = Value.ForString(Constants.SpecialSecretSig);
                    result.Fields[Constants.ValueName] = Unmarshal(secret);
                    return Value.ForStruct(result);
                },
                resource =>
                {
                    var result = new Struct();
                    result.Fields[Constants.SpecialSigKey] = Value.ForString(Constants.SpecialResourceSig);
                    result.Fields[Constants.UrnPropertyName] = Value.ForString(resource.URN);
                    result.Fields[Constants.IdPropertyName] = Unmarshal(resource.ID);
                    if (resource.PackageVersion != "")
                    {
                        result.Fields[Constants.ResourceVersionName] = Value.ForString(resource.PackageVersion);
                    }
                    return Value.ForStruct(result);
                },
                output => UnmarshalOutput(output, false),
                () => Value.ForString(Constants.UnknownValue)
            );
        }
    }

    public sealed class CheckRequest
    {
        public readonly string Urn;
        public readonly ImmutableDictionary<string, PropertyValue> Olds;
        public readonly ImmutableDictionary<string, PropertyValue> News;
        public readonly ImmutableArray<byte> RandomSeed;

        public CheckRequest(string urn, ImmutableDictionary<string, PropertyValue> olds, ImmutableDictionary<string, PropertyValue> news, ImmutableArray<byte> randomSeed)
        {
            Urn = urn;
            Olds = olds;
            News = news;
            RandomSeed = randomSeed;
        }
    }

    public sealed class CheckFailure
    {
        public string Property { get; set; }
        public string Reason { get; set; }

        public CheckFailure(string property, string reason)
        {
            Property = property;
            Reason = reason;
        }
    }

    public sealed class CheckResponse
    {
        public IDictionary<string, PropertyValue>? Inputs { get; set; }
        public IList<CheckFailure>? Failures { get; set; }
    }


    public sealed class DiffRequest
    {
        public readonly string Urn;
        public readonly string ID;
        public readonly ImmutableDictionary<string, PropertyValue> Olds;
        public readonly ImmutableDictionary<string, PropertyValue> News;
        public readonly ImmutableArray<string> IgnoreChanges;

        public DiffRequest(string urn, string id, ImmutableDictionary<string, PropertyValue> olds, ImmutableDictionary<string, PropertyValue> news, ImmutableArray<string> ignoreChanges)
        {
            Urn = urn;
            ID = id;
            Olds = olds;
            News = news;
            IgnoreChanges = ignoreChanges;
        }
    }

    public enum PropertyDiffKind
    {
        Add = 0,
        AddReplace = 1,
        Delete = 2,
        DeleteReplace = 3,
        Update = 4,
        UpdateReplace = 5,
    }

    public sealed class PropertyDiff
    {
        public PropertyDiffKind Kind { get; set; }
        public bool InputDiff { get; set; }
    }

    public sealed class DiffResponse
    {
        public bool? Changes { get; set; }

        public IList<string>? Replaces { get; set; }

        public IList<string>? Stables { get; set; }

        public bool DeleteBeforeReplace { get; set; }
        public IList<string>? Diffs { get; set; }

        public IDictionary<string, PropertyDiff>? DetailedDiff { get; set; }
    }

    public sealed class InvokeRequest
    {
        public readonly string Tok;
        public readonly ImmutableDictionary<string, PropertyValue> Args;

        public InvokeRequest(string tok, ImmutableDictionary<string, PropertyValue> args)
        {
            Tok = tok;
            Args = args;
        }
    }

    public sealed class InvokeResponse
    {

        public IDictionary<string, PropertyValue>? Return { get; set; }
        public IList<CheckFailure>? Failures { get; set; }
    }

    public sealed class GetSchemaRequest
    {
        public readonly int Version;

        public GetSchemaRequest(int version)
        {
            Version = version;
        }
    }

    public sealed class GetSchemaResponse
    {
        public string? Schema { get; set; }
    }

    public sealed class ConfigureRequest
    {
        public readonly ImmutableDictionary<string, string> Variables;
        public readonly ImmutableDictionary<string, PropertyValue> Args;
        public readonly bool AcceptSecrets;
        public readonly bool AcceptResources;

        public ConfigureRequest(ImmutableDictionary<string, string> variables, ImmutableDictionary<string, PropertyValue> args, bool acceptSecrets, bool acceptResources)
        {
            Variables = variables;
            Args = args;
            AcceptSecrets = acceptSecrets;
            AcceptResources = acceptResources;
        }
    }

    public sealed class ConfigureResponse
    {
        public bool AcceptSecrets { get; set; }
        public bool SupportsPreview { get; set; }
        public bool AcceptResources { get; set; }
        public bool AcceptOutputs { get; set; }
    }

    public sealed class GetPluginInfoResponse
    {
        public string? Version { get; set; }
    }

    public sealed class CreateRequest
    {
        public readonly string URN;
        public readonly ImmutableDictionary<string, PropertyValue> Properties;
        public readonly TimeSpan Timeout;
        public readonly bool Preview;

        public CreateRequest(string urn, ImmutableDictionary<string, PropertyValue> properties, TimeSpan timeout, bool preview)
        {
            URN = urn;
            Properties = properties;
            Timeout = timeout;
            Preview = preview;
        }
    }

    public sealed class CreateResponse
    {
        public string? ID { get; set; }
        public IDictionary<string, PropertyValue>? Properties { get; set; }
    }

    public sealed class ReadRequest {
        public readonly string ID;
        public readonly string URN;
        public readonly ImmutableDictionary<string, PropertyValue> Properties;
        public readonly ImmutableDictionary<string, PropertyValue> Inputs;

        public ReadRequest(string id, string urn, ImmutableDictionary<string, PropertyValue> properties, ImmutableDictionary<string, PropertyValue> inputs) {
            ID = id;
            URN = urn;
            Properties = properties;
            Inputs = inputs;
        }
    }

    public sealed class ReadResponse {
        public string? ID {get;set;}
        public IDictionary<string, PropertyValue>? Properties {get;set;}
        public IDictionary<string, PropertyValue>? Inputs{get;set;}
    }

    public sealed class UpdateRequest {
        public readonly string ID;
        public readonly string URN;
        public readonly ImmutableDictionary<string, PropertyValue> Olds;
        public readonly ImmutableDictionary<string, PropertyValue> News;
        public readonly TimeSpan Timeout;
        public readonly ImmutableArray<string> IgnoreChanges;
        public readonly bool Preview;

        public UpdateRequest(string id, string urn, ImmutableDictionary<string, PropertyValue> olds, ImmutableDictionary<string, PropertyValue> news, TimeSpan timeout, ImmutableArray<string> ignoreChanges, bool preview) {
            ID = id;
            URN = urn;
            Olds = olds;
            News = news;
            Timeout = timeout;
            IgnoreChanges = ignoreChanges;
            Preview = preview;
        }
    }

    public sealed class UpdateResponse {
        public IDictionary<string, PropertyValue>? Properties {get;set;}
    }

    public sealed class DeleteRequest {
        public readonly string ID;
        public readonly string URN;
        public readonly ImmutableDictionary<string, PropertyValue> Properties;
        public readonly TimeSpan Timeout;

        public DeleteRequest(string id, string urn, ImmutableDictionary<string, PropertyValue> properties, TimeSpan timeout) {
            ID = id;
            URN = urn;
            Properties = properties;
            Timeout = timeout;
        }
    }

    public abstract class Provider
    {
        public virtual Task<CheckResponse> CheckConfig(CheckRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<DiffResponse> DiffConfig(DiffRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<InvokeResponse> Invoke(InvokeRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<GetSchemaResponse> GetSchema(GetSchemaRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<ConfigureResponse> Configure(ConfigureRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<GetPluginInfoResponse> GetPluginInfo(CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task Cancel(CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<CreateResponse> Create(CreateRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<ReadResponse> Read(ReadRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<CheckResponse> Check(CheckRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<DiffResponse> Diff(DiffRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task<UpdateResponse> Update(UpdateRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public virtual Task Delete(DeleteRequest request, CancellationToken ct)
        {
            throw new NotImplementedException();
        }

        public static async Task Serve(string[] args, Func<IHost, Provider> factory, System.Threading.CancellationToken cancellationToken)
        {
            // maxRpcMessageSize raises the gRPC Max message size from `4194304` (4mb) to `419430400` (400mb)
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

                            var hostConfig = new Dictionary<string, string>();
                            if (args.Length > 0) {
                                hostConfig.Add("Host", args[0]);
                            }
                            config.AddInMemoryCollection(hostConfig);
                        })
                        .ConfigureLogging(loggingBuilder =>
                        {
                            // disable default logging
                            loggingBuilder.ClearProviders();
                        })
                        .ConfigureServices(services =>
                        {
                            // to be injected into ResourceProviderService
                            services.AddSingleton(factory);

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

            await host.WaitForShutdownAsync(cancellationToken);

            host.Dispose();
        }
    }

    class ResourceProviderService : ResourceProvider.ResourceProviderBase
    {
        readonly Func<IHost, Provider> factory;
        Provider? implementation;

        Provider Implementation
        {
            get
            {
                if (implementation == null)
                {
                    throw new RpcException(new Status(StatusCode.FailedPrecondition, "Engine host not yet attached"));
                }
                return implementation;
            }
        }

        private void CreateProvider(string address)
        {
            var host = new GrpcHost(address);
            implementation = factory(host);
        }

        public ResourceProviderService(Func<IHost, Provider> factory, IConfiguration configuration)
        {
            this.factory = factory;

            var host = configuration.GetValue<string?>("Host", null);
            if (host != null)
            {
                CreateProvider(host);
            }
        }

        public override Task<Empty> Attach(Pulumirpc.PluginAttach request, ServerCallContext context)
        {
            CreateProvider(request.Address);
            return Task.FromResult(new Empty());
        }

        public override async Task<Pulumirpc.CheckResponse> CheckConfig(Pulumirpc.CheckRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new CheckRequest(request.Urn, PropertyValue.Marshal(request.Olds), PropertyValue.Marshal(request.News), ImmutableArray.ToImmutableArray(request.RandomSeed));
                var domResponse = await Implementation.CheckConfig(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.CheckResponse();
                grpcResponse.Inputs = domResponse.Inputs == null ? null : PropertyValue.Unmarshal(domResponse.Inputs);
                if (domResponse.Failures != null)
                {
                    foreach (var domFailure in domResponse.Failures)
                    {
                        var grpcFailure = new Pulumirpc.CheckFailure();
                        grpcFailure.Property = domFailure.Property;
                        grpcFailure.Reason = domFailure.Reason;
                        grpcResponse.Failures.Add(grpcFailure);
                    }
                }
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.DiffResponse> DiffConfig(Pulumirpc.DiffRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new DiffRequest(request.Urn, request.Id, PropertyValue.Marshal(request.Olds), PropertyValue.Marshal(request.News), request.IgnoreChanges.ToImmutableArray());
                var domResponse = await Implementation.DiffConfig(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.DiffResponse();
                if (domResponse.Changes.HasValue)
                {
                    grpcResponse.Changes = domResponse.Changes.Value ? Pulumirpc.DiffResponse.Types.DiffChanges.DiffSome : Pulumirpc.DiffResponse.Types.DiffChanges.DiffNone;
                }
                if (domResponse.Stables != null)
                {
                    grpcResponse.Stables.AddRange(domResponse.Stables);
                }
                if (domResponse.Replaces != null)
                {
                    grpcResponse.Replaces.AddRange(domResponse.Replaces);
                }
                grpcResponse.DeleteBeforeReplace = domResponse.DeleteBeforeReplace;
                if (domResponse.Diffs != null)
                {
                    grpcResponse.Diffs.AddRange(domResponse.Diffs);
                }
                if (domResponse.DetailedDiff != null)
                {
                    foreach (var item in domResponse.DetailedDiff)
                    {
                        var domDiff = item.Value;
                        var grpcDiff = new Pulumirpc.PropertyDiff();
                        grpcDiff.InputDiff = domDiff.InputDiff;
                        grpcDiff.Kind = (Pulumirpc.PropertyDiff.Types.Kind)domDiff.Kind;
                        grpcResponse.DetailedDiff.Add(item.Key, grpcDiff);
                    }
                }
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.InvokeResponse> Invoke(Pulumirpc.InvokeRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new InvokeRequest(request.Tok, PropertyValue.Marshal(request.Args));
                var domResponse = await Implementation.Invoke(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.InvokeResponse();
                grpcResponse.Return = domResponse.Return == null ? null : PropertyValue.Unmarshal(domResponse.Return);
                if (domResponse.Failures != null)
                {
                    foreach (var domFailure in domResponse.Failures)
                    {
                        var grpcFailure = new Pulumirpc.CheckFailure();
                        grpcFailure.Property = domFailure.Property;
                        grpcFailure.Reason = domFailure.Reason;
                        grpcResponse.Failures.Add(grpcFailure);
                    }
                }
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.GetSchemaResponse> GetSchema(Pulumirpc.GetSchemaRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new GetSchemaRequest(request.Version);
                var domResponse = await Implementation.GetSchema(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.GetSchemaResponse();
                grpcResponse.Schema = domResponse.Schema ?? "";
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.ConfigureResponse> Configure(Pulumirpc.ConfigureRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new ConfigureRequest(request.Variables.ToImmutableDictionary(), PropertyValue.Marshal(request.Args), request.AcceptSecrets, request.AcceptResources);
                var domResponse = await Implementation.Configure(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.ConfigureResponse();
                grpcResponse.AcceptSecrets = domResponse.AcceptSecrets;
                grpcResponse.SupportsPreview = domResponse.SupportsPreview;
                grpcResponse.AcceptResources = domResponse.AcceptResources;
                grpcResponse.AcceptOutputs = domResponse.AcceptOutputs;
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.PluginInfo> GetPluginInfo(Empty request, ServerCallContext context)
        {
            try
            {
                var domResponse = await Implementation.GetPluginInfo(context.CancellationToken);
                var grpcResponse = new Pulumirpc.PluginInfo();
                grpcResponse.Version = domResponse.Version ?? "";
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Empty> Cancel(Empty request, ServerCallContext context)
        {
            try
            {
                await Implementation.Cancel(context.CancellationToken);
                return new Empty();
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.CreateResponse> Create(Pulumirpc.CreateRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new CreateRequest(request.Urn, PropertyValue.Marshal(request.Properties), TimeSpan.FromSeconds(request.Timeout), request.Preview);
                var domResponse = await Implementation.Create(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.CreateResponse();
                grpcResponse.Id = domResponse.ID ?? "";
                grpcResponse.Properties = domResponse.Properties == null ? null : PropertyValue.Unmarshal(domResponse.Properties);
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.ReadResponse> Read(Pulumirpc.ReadRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new ReadRequest(request.Id, request.Urn, PropertyValue.Marshal(request.Properties), PropertyValue.Marshal(request.Inputs));
                var domResponse = await Implementation.Read(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.ReadResponse();
                grpcResponse.Id = domResponse.ID ?? "";
                grpcResponse.Properties = domResponse.Properties == null ? null : PropertyValue.Unmarshal(domResponse.Properties);
                grpcResponse.Inputs = domResponse.Inputs == null ? null : PropertyValue.Unmarshal(domResponse.Inputs);
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.CheckResponse> Check(Pulumirpc.CheckRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new CheckRequest(request.Urn, PropertyValue.Marshal(request.Olds), PropertyValue.Marshal(request.News), ImmutableArray.ToImmutableArray(request.RandomSeed));
                var domResponse = await Implementation.Check(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.CheckResponse();
                grpcResponse.Inputs = domResponse.Inputs == null ? null : PropertyValue.Unmarshal(domResponse.Inputs);
                if (domResponse.Failures != null)
                {
                    foreach (var domFailure in domResponse.Failures)
                    {
                        var grpcFailure = new Pulumirpc.CheckFailure();
                        grpcFailure.Property = domFailure.Property;
                        grpcFailure.Reason = domFailure.Reason;
                        grpcResponse.Failures.Add(grpcFailure);
                    }
                }
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.DiffResponse> Diff(Pulumirpc.DiffRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new DiffRequest(request.Urn, request.Id, PropertyValue.Marshal(request.Olds), PropertyValue.Marshal(request.News), request.IgnoreChanges.ToImmutableArray());
                var domResponse = await Implementation.Diff(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.DiffResponse();
                if (domResponse.Changes.HasValue)
                {
                    grpcResponse.Changes = domResponse.Changes.Value ? Pulumirpc.DiffResponse.Types.DiffChanges.DiffSome : Pulumirpc.DiffResponse.Types.DiffChanges.DiffNone;
                }
                if (domResponse.Stables != null)
                {
                    grpcResponse.Stables.AddRange(domResponse.Stables);
                }
                if (domResponse.Replaces != null)
                {
                    grpcResponse.Replaces.AddRange(domResponse.Replaces);
                }
                grpcResponse.DeleteBeforeReplace = domResponse.DeleteBeforeReplace;
                if (domResponse.Diffs != null)
                {
                    grpcResponse.Diffs.AddRange(domResponse.Diffs);
                }
                if (domResponse.DetailedDiff != null)
                {
                    foreach (var item in domResponse.DetailedDiff)
                    {
                        var domDiff = item.Value;
                        var grpcDiff = new Pulumirpc.PropertyDiff();
                        grpcDiff.InputDiff = domDiff.InputDiff;
                        grpcDiff.Kind = (Pulumirpc.PropertyDiff.Types.Kind)domDiff.Kind;
                        grpcResponse.DetailedDiff.Add(item.Key, grpcDiff);
                    }
                }
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Pulumirpc.UpdateResponse> Update(Pulumirpc.UpdateRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new UpdateRequest(request.Urn, request.Id, PropertyValue.Marshal(request.Olds), PropertyValue.Marshal(request.News), TimeSpan.FromSeconds(request.Timeout), request.IgnoreChanges.ToImmutableArray(), request.Preview);
                var domResponse = await Implementation.Update(domRequest, context.CancellationToken);
                var grpcResponse = new Pulumirpc.UpdateResponse();
                grpcResponse.Properties = domResponse.Properties == null ? null : PropertyValue.Unmarshal(domResponse.Properties);
                return grpcResponse;
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override async Task<Empty> Delete(Pulumirpc.DeleteRequest request, ServerCallContext context)
        {
            try
            {
                var domRequest = new DeleteRequest(request.Urn, request.Id, PropertyValue.Marshal(request.Properties), TimeSpan.FromSeconds(request.Timeout));
                await Implementation.Delete(domRequest, context.CancellationToken);
                return new Empty();
            }
            catch (NotImplementedException ex)
            {
                throw new RpcException(new Status(StatusCode.Unimplemented, ex.Message));
            }
            catch (TaskCanceledException ex)
            {
                throw new RpcException(new Status(StatusCode.Cancelled, ex.Message));
            }
            catch (Exception ex)
            {
                throw new RpcException(new Status(StatusCode.Internal, ex.Message));
            }
        }

        public override Task<Pulumirpc.ConstructResponse> Construct(Pulumirpc.ConstructRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "Component resources not yet supported"));
        }

        public override Task<Pulumirpc.CallResponse> Call(Pulumirpc.CallRequest request, ServerCallContext context)
        {
            throw new RpcException(new Status(StatusCode.Unimplemented, "Component resources not yet supported"));
        }
    }
}
