using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Awsx = Pulumi.Awsx;

return await Deployment.RunAsync(() => 
{
    var vpc = new Awsx.Ec2.Vpc("vpc", new()
    {
        SubnetSpecs = new()
        {
            new Awsx.Ec2.Inputs.SubnetSpecArgs
            {
                Type = Awsx.Ec2.SubnetType.Public,
                CidrMask = 22,
            },
            new Awsx.Ec2.Inputs.SubnetSpecArgs
            {
                Type = Awsx.Ec2.SubnetType.Private,
                CidrMask = 20,
            },
        },
    });

});

