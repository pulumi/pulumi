using System.Collections.Generic;
using System.Linq;
using Pulumi;

return await Deployment.RunAsync(() => 
{
    var stackRef = new Pulumi.StackReference("stackRef", new()
    {
        Name = "foo/bar/dev",
    });

});

