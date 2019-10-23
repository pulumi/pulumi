// Copyright 2016-2019, Pulumi Corporation

#nullable enable


namespace Pulumi.Serialization
{
    internal static class OutputData
    {
        public static OutputData<X> Create<X>(X value, bool isKnown, bool isSecret)
            => new OutputData<X>(value, isKnown, isSecret);

        public static (bool isKnown, bool isSecret) Combine<X>(OutputData<X> data, bool isKnown, bool isSecret)
           => (isKnown && data.IsKnown, isSecret || data.IsSecret);
    }

    internal struct OutputData<X>
    {
        public readonly X Value;
        public readonly bool IsKnown;
        public readonly bool IsSecret;

        public OutputData(X value, bool isKnown, bool isSecret)
        {
            Value = value;
            IsKnown = isKnown;
            IsSecret = isSecret;
        }

        public static implicit operator OutputData<object?>(OutputData<X> data)
            => new OutputData<object?>(data.Value, data.IsKnown, data.IsSecret);

        public void Deconstruct(out X value, out bool isKnown, out bool isSecret)
        {
            value = Value;
            isKnown = IsKnown;
            isSecret = IsSecret;
        }
    }
}