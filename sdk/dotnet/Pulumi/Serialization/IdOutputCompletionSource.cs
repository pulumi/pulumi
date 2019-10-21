// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    internal sealed class IdOutputCompletionSource : OutputCompletionSource<Id>
    {
        public IdOutputCompletionSource(Resource resource)
            : base(resource)
        {
        }

        public void SetResult(string id)
            => SetResult(new OutputData<Id>(new Id(id), isKnown: true, isSecret: false));

        public void SetUnknownResult()
            => SetResult(new OutputData<Id>(new Id(""), isKnown: false, isSecret: false));
    }
}
