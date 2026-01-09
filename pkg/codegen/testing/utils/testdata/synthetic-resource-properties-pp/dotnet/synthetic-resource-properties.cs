using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Synthetic = Pulumi.Synthetic.Synthetic;

return await Deployment.RunAsync(() => 
{
    var rt = new Synthetic.ResourceProperties.Root("rt");

    return new Dictionary<string, object?>
    {
        ["trivial"] = rt,
        ["simple"] = rt.Res1,
        ["foo"] = rt.Res1.Apply(res1 => res1.Obj1?.Res2?.Obj2),
        ["complex"] = rt.Res1.Apply(res1 => res1.Obj1?.Res2?.Obj2?.Answer),
    };
});

