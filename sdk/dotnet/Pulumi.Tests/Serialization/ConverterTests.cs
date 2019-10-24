// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;

namespace Pulumi.Tests.Serialization
{
    public abstract class ConverterTests : PulumiTest
    {
        protected static readonly Value UnknownValue = new Value { StringValue = Constants.UnknownValue };

        protected static Value CreateSecretValue(Value value)
            => new Value
            {
                StructValue = new Struct
                {
                    Fields =
                    {
                        { Constants.SpecialSigKey, new Value { StringValue = Constants.SpecialSecretSig } },
                        { Constants.SecretValueName, value },
                    }
                }
            };

        protected Output<T> CreateUnknownOutput<T>(T value)
            => new Output<T>(ImmutableHashSet<Resource>.Empty, Task.FromResult(new OutputData<T>(value, isKnown: false, isSecret: false)));

        protected async Task<Value> SerializeToValueAsync(object? value)
        {
            var serializer = new Serializer(excessiveDebugOutput: false);
            return Serializer.CreateValue(
                await serializer.SerializeAsync(ctx: "", value).ConfigureAwait(false));
        }
    }
}
