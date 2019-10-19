// Copyright 2016-2018, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    public sealed class StringOutputCompletionSource : ProtobufCompletionSource<string>
    {
        public StringOutputCompletionSource(Resource resource)
            : base(resource, Deserializers.StringDeserializer)
        {
        }
    }
}
