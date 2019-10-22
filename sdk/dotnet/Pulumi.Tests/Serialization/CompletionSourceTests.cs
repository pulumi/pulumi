// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using Google.Protobuf.WellKnownTypes;
using Pulumi.Rpc;

namespace Pulumi.Tests.Serialization
{
    public abstract class CompletionSourceTests : PulumiTest
    {
        protected static readonly Value UnknownValue = new Value { StringValue = Constants.UnknownValue };

        protected static Value CreateSecret(Value value)
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
    }
}
