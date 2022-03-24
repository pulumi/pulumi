using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    public static class Rpc {

        public static ImmutableDictionary<string, object?> DeserialiseProperties(Struct properties)
        {
            var output = Deserializer.Deserialize(Value.ForStruct(properties));
            if (!output.IsKnown || output.IsSecret)
            {
                throw new Exception("Deserialize of a Struct should always be known and not secret!");
            }

            var result = output.Value as ImmutableDictionary<string, object?>;
            if (result == null)
            {
                throw new Exception("Deserialize of a Struct should always return an ImmutableDictionary!");
            }

            return result;
        }

        public static Struct SerialiseProperties(IDictionary<string, object?> properties)
        {
            var dictionary = ImmutableDictionary.CreateRange(properties);
            return Serializer.CreateStruct(dictionary);
        }
    }
}