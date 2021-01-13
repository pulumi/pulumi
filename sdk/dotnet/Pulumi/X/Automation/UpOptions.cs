using System;
using System.Collections.Generic;
using System.Reflection;

namespace Pulumi.X.Automation
{
    /// <summary>
    /// Options controlling the behavior of an <see cref="XStack.UpAsync(UpOptions, System.Threading.CancellationToken)"/> operation.
    /// </summary>
    public sealed class UpOptions : UpdateOptions
    {
        public bool? ExpectNoChanges { get; set; }

        public List<string>? Replace { get; set; }

        public bool? TargetDependents { get; set; }

        public Action<string>? OnOutput { get; set; }

        public PulumiFn? Program { get; set; }

        /// <summary>
        /// The collection of assemblies containing all necessary <see cref="CustomResource"/> implementations.
        /// Only comes into play during inline program execution, when a <see cref="Program"/> is provided.
        /// <para/>
        /// Useful when control is needed over what assemblies Pulumi should search for <see cref="CustomResource"/>
        /// discovery - such as when the executing assembly does not itself reference the assemblies that contain
        /// those implementations.
        /// <para/>
        /// If not provided, <see cref="AppDomain.CurrentDomain"/> and its referenced assemblies will be used
        /// for <see cref="CustomResource"/> discovery.
        /// </summary>
        public IList<Assembly>? ResourcePackageAssemblies { get; set; }
    }
}
