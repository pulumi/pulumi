// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;

namespace Pulumi.Rpc
{
    public sealed class ListOutputCompletionSource<T> : ProtobufCompletionSource<ImmutableArray<T>>
    {
        public ListOutputCompletionSource(Resource resource, Deserializer<T> elementDeserializer)
            : base(resource, Deserializers.CreateListDeserializer(elementDeserializer))
        {
        }
    }
}
