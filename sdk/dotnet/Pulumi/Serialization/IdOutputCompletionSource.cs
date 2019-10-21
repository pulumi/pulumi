// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Rpc
{
    internal sealed class IdOutputCompletionSource : OutputCompletionSource<string>
    {
        public IdOutputCompletionSource(Resource resource)
            : base(resource)
        {
        }

        public void SetResult(string id)
            => SetResult(new OutputData<string>(id, isKnown: true, isSecret: false));

        public void SetUnknownResult()
            => SetResult(new OutputData<string>("", isKnown: false, isSecret: false));
    }
}
