// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    public sealed class BoolOutputCompletionSource : ProtobufOutputCompletionSource<bool>
    {
        public BoolOutputCompletionSource(Resource resource)
            : base(resource, Deserializers.BoolDeserializer)
        {
        }
    }
}
