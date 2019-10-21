// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    internal sealed class UrnOutputCompletionSource : OutputCompletionSource<Urn>
    {
        public UrnOutputCompletionSource(Resource resource)
            : base(resource)
        {
        }

        public void SetResult(string urn)
            => SetResult(new OutputData<Urn>(new Urn(urn), isKnown: true, isSecret: false));
    }
}
