// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    internal interface IProtobufOutputCompletionSource : IOutputCompletionSource
    {
        void SetResult(Value value);
    }
}
