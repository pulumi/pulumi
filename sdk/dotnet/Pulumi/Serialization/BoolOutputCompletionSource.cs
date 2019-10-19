// Copyright 2016-2018, Pulumi Corporation

#nullable enable


namespace Pulumi.Rpc
{
    public sealed class BoolOutputCompletionSource : ProtobufCompletionSource<bool>
    {
        public BoolOutputCompletionSource(Resource resource)
            : base(resource, Deserializers.BoolDeserializer)
        {
        }
    }
}
