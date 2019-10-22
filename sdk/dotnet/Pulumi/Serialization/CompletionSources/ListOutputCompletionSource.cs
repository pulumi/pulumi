// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Immutable;

namespace Pulumi.Serialization
{
    public sealed class ListOutputCompletionSource<T> : ProtobufOutputCompletionSource<ImmutableArray<T>>
    {
        public ListOutputCompletionSource(Resource? resource, Deserializer<T> elementDeserializer)
            : base(resource, Deserializers.CreateListDeserializer(elementDeserializer))
        {
        }
    }
}
