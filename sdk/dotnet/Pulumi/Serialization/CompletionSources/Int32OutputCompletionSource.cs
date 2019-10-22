// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Serialization
{
    public sealed class Int32OutputCompletionSource : ProtobufOutputCompletionSource<int>
    {
        public Int32OutputCompletionSource(Resource resource)
            : base(resource, Deserializers.Int32Deserializer)
        {
        }
    }
}
