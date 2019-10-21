// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    public sealed class StringOutputCompletionSource : ProtobufOutputCompletionSource<string>
    {
        public StringOutputCompletionSource(Resource resource)
            : base(resource, Deserializers.StringDeserializer)
        {
        }
    }
}
