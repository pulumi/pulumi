namespace Pulumi.Automation
{
    /// <summary>
    /// Various configuration options that apply to different language runtimes.
    /// </summary>
    public class ProjectRuntimeOptions
    {
        /// <summary>
        /// Applies to NodeJS projects only.
        /// <para/>
        /// A boolean that controls whether to use ts-node to execute sources.
        /// </summary>
        public bool? TypeScript { get; set; }

        /// <summary>
        /// Applies to Go and .NET project only.
        /// <para/>
        /// Go: A string that specifies the name of a pre-build executable to look for on your path.
        /// <para/>
        /// .NET: A string that specifies the path of a pre-build .NET assembly.
        /// </summary>
        public string? Binary { get; set; }

        /// <summary>
        /// Applies to Python projects only.
        /// <para/>
        /// A string that specifies the path to a virtual environment to use when running the program.
        /// </summary>
        public string? VirtualEnv { get; set; }
    }
}
