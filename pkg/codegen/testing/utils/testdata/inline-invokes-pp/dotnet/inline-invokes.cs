using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var webSecurityGroup = new Aws.Ec2.SecurityGroup("webSecurityGroup", new()
    {
        VpcId = Aws.Ec2.GetVpc.Invoke(new()
        {
            Default = true,
        }).Apply(invoke => invoke.Id),
    });

});

