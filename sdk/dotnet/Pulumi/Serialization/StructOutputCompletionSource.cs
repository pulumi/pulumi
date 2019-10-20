// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;

namespace Pulumi.Rpc
{
    public sealed class StructOutputCompletionSource<T> : ProtobufOutputCompletionSource<ImmutableDictionary<string, T>>
    {
        public StructOutputCompletionSource(Resource resource, Deserializer<T> elementDeserializer)
            : base(resource, Deserializers.CreateStructDeserializer(elementDeserializer))
        {
        }
    }
}
