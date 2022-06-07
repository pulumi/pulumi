namespace Pulumi.Automation
{
    public class PluginInstallOptions
    {
        /// <summary>
        /// If <c>true</c>, force installation of an exact version match (usually >= is accepted).
        /// <para/>
        /// Defaults to <c>false</c>.
        /// </summary>
        public bool ExactVersion { get; set; }

        /// <summary>
        /// A URL to download plugins from.
        /// </summary>
        public string? ServerUrl { get; set; }
    }
}
