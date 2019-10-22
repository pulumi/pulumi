// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using Google.Protobuf.WellKnownTypes;

namespace Pulumi.Serialization
{
    public delegate OutputData<T> Deserializer<T>(Value value);
}