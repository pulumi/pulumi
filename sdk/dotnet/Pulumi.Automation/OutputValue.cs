// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public sealed class OutputValue
    {
        public object Value { get; }

        public bool IsSecret { get; }

        internal OutputValue(
            object value,
            bool isSecret)
        {
            this.Value = value;
            this.IsSecret = isSecret;
        }
    }
}
