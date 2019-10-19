// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Generic;

namespace Pulumi.Rpc
{
    public class ComplexOutputCompletionSource<T> : ProtobufOutputCompletionSource<T>
    {
        public ComplexOutputCompletionSource(
            Resource resource,
            Func<IDictionary<string, object>, T> convert)
            : base(resource, CreateDeserializeFunction(convert))
        {
        }

        private static Deserializer<T> CreateDeserializeFunction(Func<IDictionary<string, object>, T> convert)
            => v =>
            {
                var (unwrapped, isSecret) = Deserializers.GenericStructDeserializer(v);
                return (convert(unwrapped), isSecret);
            };
    }
}
