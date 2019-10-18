// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Diagnostics;
using System.Diagnostics.Contracts;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumirpc;

namespace Pulumi.Rpc
{
    public abstract class AbstractRegistrationTaskCompletionSource<T>
    {
        private readonly TaskCompletionSource<OutputData<T>> _taskCompletionSource = new TaskCompletionSource<OutputData<T>>();

        protected AbstractRegistrationTaskCompletionSource()
        {
            Output = new Output<T>(_taskCompletionSource.Task);
        }

        public Output<T> Output { get; }

        internal void Assign(Task<RegisterResourceResponse> response, string fieldName)
        {
            response.Assign(_taskCompletionSource, r => Extract(r, fieldName));
        }

        private OutputData<T> Extract(RegisterResourceResponse response, string fieldName)
        {
            if (response?.Object?.Fields == null ||
                !response.Object.Fields.TryGetValue(fieldName, out var value))
            {
                return new OutputData<T>(default!, isKnown: !Deployment.Instance.Options.DryRun, isSecret: false);
            }

            UnwrapSecret(value, out var isSecret, out var unwrapped);
            var converted = Convert(value);
            return new OutputData<T>(converted, converted != null)
        }

        protected static void UnwrapSecret(Value value, out bool isSecret, out Value unwrapped)
        {
            if (value?.KindCase == Value.KindOneofCase.StructValue &&
                value.StructValue.Fields.TryGetValue(Constants.SpecialSigKey, out var sigValue) &&
                sigValue.KindCase == Value.KindOneofCase.StringValue &&
                sigValue.StringValue == Constants.SpecialSecretSig)
            {
                isSecret = true;
                Debug.Assert(value.StructValue.Fields.TryGetValue("value", out unwrapped));
                return;
            }

            isSecret = false;
            unwrapped = value;
        }
    }

    public sealed class BoolRegistrationTaskCompletionSource<T> : AbstractRegistrationTaskCompletionSource<bool>
    {
        private protected override OutputData<bool> Extract(RegisterResourceResponse response)
        {
            throw new NotImplementedException();
        }
    }
}
