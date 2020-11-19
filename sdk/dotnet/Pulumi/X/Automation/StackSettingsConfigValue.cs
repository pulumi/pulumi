namespace Pulumi.X.Automation
{
    public class StackSettingsConfigValue
    {
        public string Value { get; }

        public bool IsSecure { get; }

        public bool IsObject { get; }

        public StackSettingsConfigValue(
            string value,
            bool isSecure,
            bool isObject)
        {
            this.Value = value;
            this.IsSecure = isSecure;
            this.IsObject = isObject;
        }
    }
}
