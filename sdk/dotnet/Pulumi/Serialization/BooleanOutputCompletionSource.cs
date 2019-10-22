// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    public sealed class BooleanOutputCompletionSource : ProtobufOutputCompletionSource<bool>
    {
        public BooleanOutputCompletionSource(Resource? resource)
            : base(resource, Deserializers.BoolDeserializer)
        {
        }
    }
}
