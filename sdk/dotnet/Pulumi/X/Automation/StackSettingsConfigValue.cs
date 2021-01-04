namespace Pulumi.X.Automation
{
    public class StackSettingsConfigValue
    {
        public string Value { get; }

        public bool IsSecure { get; }

        public StackSettingsConfigValue(
            string value,
            bool isSecure)
        {
            this.Value = value;
            this.IsSecure = isSecure;
        }
    }
}
