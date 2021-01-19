using System.Collections.Generic;

namespace Pulumi.Automation
{
    /// <summary>
    /// A Pulumi program as an inline function (in process).
    /// </summary>
    public delegate IDictionary<string, object?> PulumiFn();
}
