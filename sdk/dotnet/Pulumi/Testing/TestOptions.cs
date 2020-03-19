namespace Pulumi.Testing
{
    /// <summary>
    /// Optional settings for <see cref="Deployment.TestAsync{T}"/>.
    /// </summary>
    public class TestOptions
    {
        /// <summary>
        /// Project name. Defaults to <b>"project"</b> if not specified.
        /// </summary>
        public string? ProjectName { get; set; }
        
        /// <summary>
        /// Stack name. Defaults to <b>"stack"</b> if not specified.
        /// </summary>
        public string? StackName { get; set; }
        
        /// <summary>
        /// Whether the test runs in Preview mode. Defaults to <b>true</b> if not specified.
        /// </summary>
        public bool? IsPreview { get; set; }
    }
}
