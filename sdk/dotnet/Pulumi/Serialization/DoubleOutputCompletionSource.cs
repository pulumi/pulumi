// Copyright 2016-2018, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    public sealed class DoubleOutputCompletionSource : ProtobufOutputCompletionSource<double>
    {
        public DoubleOutputCompletionSource(Resource resource)
            : base(resource, Deserializers.DoubleDeserializer)
        {
        }
    }
}
