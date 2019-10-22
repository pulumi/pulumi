// Copyright 2016-2019, Pulumi Corporation

#nullable enable

namespace Pulumi.Serialization
{
    internal sealed class IdOutputCompletionSource : OutputCompletionSource<string>
    {
        public IdOutputCompletionSource(Resource resource)
            : base(resource)
        {
        }

        public void SetResult(string id)
            => SetResult(OutputData.Create(id, isKnown: true, isSecret: false));

        public void SetUnknownResult()
            => SetResult(OutputData.Create("", isKnown: false, isSecret: false));
    }
}
