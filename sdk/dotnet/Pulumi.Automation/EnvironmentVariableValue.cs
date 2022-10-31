// Copyright 2016-2022, Pulumi Corporation

namespace Pulumi.Automation
{
    public class EnvironmentVariableValue
    {
        public string Value { get; set; }

        public bool IsSecret { get; set; }

        public EnvironmentVariableValue(
            string value,
            bool isSecret = false)
        {
            Value = value;
            IsSecret = isSecret;
        }
    }
}
