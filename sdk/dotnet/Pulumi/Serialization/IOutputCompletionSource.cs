// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi.Rpc
{
    internal interface IOutputCompletionSource
    {
        void TrySetException(Exception exception);
        void SetDefaultResult(bool isKnown);
    }
}
