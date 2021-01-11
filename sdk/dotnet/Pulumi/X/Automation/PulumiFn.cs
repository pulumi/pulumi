using System.Collections.Generic;
using System.Threading.Tasks;

namespace Pulumi.X.Automation
{
    /// <summary>
    /// A Pulumi program as an inline function (in process).
    /// </summary>
    public delegate IDictionary<string, object?> PulumiFn();
}
