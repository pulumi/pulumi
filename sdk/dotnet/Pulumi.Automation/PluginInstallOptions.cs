namespace Pulumi.Automation
{
    public class PluginInstallOptions
    {
        /// <summary>
        /// If <c>true</c>, force installation of an exact version match (usually >= is accepted).
        /// <para/>
        /// Defaults to <c>false</c>.
        /// </summary>
        public bool Exact { get; set; } = false;

        /// <summary>
        /// A URL to download plugins from.
        /// </summary>
        public string? Server { get; set; }
    }
}
