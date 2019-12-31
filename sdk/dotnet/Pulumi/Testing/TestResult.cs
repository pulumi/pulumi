using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;

namespace Pulumi.Testing
{
    /// <summary>
    /// Represents an outcome of <see
    /// cref="Deployment.TestAsync{TStack}(ITestContext)"/>.
    /// </summary>
    public class TestResult
    {
        /// <summary>
        /// Whether the test run failed with an error.
        /// </summary>
        public bool HasErrors { get; }

        /// <summary>
        /// Error messages that were logged during the run.
        /// </summary>
        public ImmutableArray<string> LoggedErrors { get; }

        /// <summary>
        /// All Pulumi resources that got registered during the run.
        /// </summary>
        public ImmutableArray<Resource> Resources { get; }

        internal TestResult(bool hasErrors, IEnumerable<string> loggedErrors, IEnumerable<Resource> resources)
        {
            this.HasErrors = hasErrors;
            this.LoggedErrors = loggedErrors.ToImmutableArray();
            this.Resources = resources.ToImmutableArray();
        }
    }

}
