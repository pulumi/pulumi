using System.Collections.Generic;

namespace Pulumi.X.Automation
{
    public class StackSettingsConfigValue
    {
        public string? ValueString { get; }

        public IDictionary<string, object>? ValueObject { get; }

        public bool IsSecure { get; }

        public bool IsObject { get; }

        public StackSettingsConfigValue(
            string value,
            bool isSecure)
            : this(value, null, isSecure, false)
        {
        }

        public StackSettingsConfigValue(
            IDictionary<string, object> value)
            : this(null, value, false, true)
        {
        }

        private StackSettingsConfigValue(
            string? valueString,
            IDictionary<string, object>? valueObject,
            bool isSecure,
            bool isObject)
        {
            this.ValueString = valueString;
            this.ValueObject = valueObject;
            this.IsSecure = isSecure;
            this.IsObject = isObject;
        }
    }
}
