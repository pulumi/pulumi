// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    internal sealed class UrnOutputCompletionSource : OutputCompletionSource<string>
    {
        public UrnOutputCompletionSource(Resource resource)
            : base(resource)
        {
        }

        public void SetResult(string urn)
            => SetResult(new OutputData<string>(urn, isKnown: true, isSecret: false));
    }
}
